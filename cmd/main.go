package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vivgrid/yc/pkg"
	"github.com/yomorun/yomo"
	"github.com/yomorun/yomo/serverless"
)

var (
	target     string
	zipperAddr string
	secret     string // app secret
	tool       string // sfn name
	meshNum    uint32 = 3
	resCount   atomic.Uint32
	resErr     atomic.Value
	cancel     context.CancelFunc
	envs       []string
)

// resetResponseState clears the per-command counters before issuing a request.
func resetResponseState() {
	resCount.Store(0)
	resErr.Store("")
}

// lastError retrieves the latest error message recorded by the handler.
func lastError() string {
	if v := resErr.Load(); v != nil {
		if msg, ok := v.(string); ok {
			return msg
		}
	}
	return ""
}

// normalizeZipperAddr ensures the zipper address has a port.
// If no port is specified, it defaults to 9000.
func normalizeZipperAddr(addr string) string {
	if addr == "" {
		return "zipper.vivgrid.com:9000"
	}
	// If the address already contains a port, return as-is
	if strings.Contains(addr, ":") {
		return addr
	}
	// If no port specified, add default port 9000
	return addr + ":9000"
}

func addVersionCmd(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("yc version:", pkg.CliVersion)
		},
	}
	rootCmd.AddCommand(cmd)

	return cmd
}

func addUploadCmd(rootCmd *cobra.Command) *cobra.Command {
	msg := &pkg.ReqMsgUpload{}
	cmd := &cobra.Command{
		Use:   "upload src_file[.go|.zip|dir]",
		Short: "Upload the source code and compile",
		Args:  cobra.ExactArgs(1),
		Run: run(
			pkg.TAG_REQUEST_UPLOAD,
			msg,
			func(args []string) error {
				src := args[0]
				info, err := os.Stat(src)
				if err != nil {
					return err
				}

				var data []byte

				if info.IsDir() {
					f, err := os.CreateTemp("", "yc-*.zip")
					if err != nil {
						return err
					}
					zipPath := f.Name()
					defer os.Remove(zipPath)
					defer f.Close()

					// Create custom ToZip function with exclusions
					err = ZipWithExclusions(src, zipPath)
					if err != nil {
						return err
					}

					data, err = os.ReadFile(zipPath)
					if err != nil {
						return err
					}
				} else {
					switch path.Ext(src) {
					case ".zip":
						data, err = os.ReadFile(src)
						if err != nil {
							return err
						}
					case ".go":
						buf := new(bytes.Buffer)
						writer := zip.NewWriter(buf)

						f, err := writer.Create("app.go")
						if err != nil {
							return err
						}

						content, err := os.ReadFile(src)
						if err != nil {
							return err
						}

						_, err = f.Write(content)
						if err != nil {
							return err
						}

						writer.Close()
						data = buf.Bytes()
					default:
						return errors.New("unsupported src file type")
					}
				}

				msg.ZipData = data

				return nil
			},
		),
		GroupID: groupIDGeneral,
	}
	rootCmd.AddCommand(cmd)

	return cmd
}

func addCreateCmd(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create serverless deployment and start it",
		Args:  cobra.ExactArgs(0),
		Run: run(
			pkg.TAG_REQUEST_CREATE,
			&pkg.ReqMsgCreate{
				Envs: &envs,
			},
			nil,
		),
		GroupID: groupIDDeployment,
	}
	rootCmd.AddCommand(cmd)
	cmd.Flags().StringArrayVar(&envs, "env", nil, "Set environment variable")

	return cmd
}

func addRemoveCmd(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Delete current serverless deployment",
		Args:  cobra.ExactArgs(0),
		Run: run(
			pkg.TAG_REQUEST_REMOVE,
			&pkg.ReqMsgRemove{},
			nil,
		),
		GroupID: groupIDDeployment,
	}
	rootCmd.AddCommand(cmd)

	return cmd
}

func addStatusCmd(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show serverless status",
		Args:  cobra.ExactArgs(0),
		Run: run(
			pkg.TAG_REQUEST_STATUS,
			&pkg.ReqMsgStatus{},
			nil,
		),
		GroupID: groupIDMonitoring,
	}
	rootCmd.AddCommand(cmd)

	return cmd
}

func addLogsCmd(rootCmd *cobra.Command) *cobra.Command {
	var tail int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Observe serverless logs in real-time",
		Args:  cobra.ExactArgs(0),
		Run: run(
			pkg.TAG_REQUEST_LOGS,
			&pkg.ReqMsgLogs{},
			nil,
		),
		GroupID: groupIDMonitoring,
	}
	rootCmd.AddCommand(cmd)
	cmd.Flags().IntVar(&tail, "tail", 20, "Tail logs")

	return cmd
}

func run[T any](tag uint32, reqMsg *T, f func([]string) error) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		resetResponseState()
		sfn := yomo.NewStreamFunction("yc-response", zipperAddr, yomo.WithSfnCredential(secret))
		sfn.SetHandler(Handler)
		sfn.SetObserveDataTags(pkg.ResponseTag(tag))
		sfn.SetWantedTarget(target)
		err := sfn.Connect()
		if err != nil {
			fmt.Println("sfn connect to zipper error:", err)
			os.Exit(1)
		}
		defer sfn.Close()

		source := yomo.NewSource("yc-request", zipperAddr, yomo.WithCredential(secret))
		err = source.Connect()
		if err != nil {
			fmt.Println("source connect to zipper error:", err)
			os.Exit(1)
		}
		defer source.Close()

		if f != nil {
			err = f(args)
			if err != nil {
				fmt.Println("exec cmd error:", err)
				os.Exit(1)
			}
		}

		req := &pkg.Request[T]{
			Version: pkg.SpecVersion,
			Target:  target,
			SfnName: tool,
			Msg:     reqMsg,
		}

		buf, _ := json.Marshal(req)

		var ctx context.Context
		switch tag {
		case pkg.TAG_REQUEST_LOGS:
			ctx, cancel = context.WithCancel(context.Background())
			go func() {
				for {
					source.Write(tag, buf)
					time.Sleep(time.Second * 15)
				}
			}()
		case pkg.TAG_REQUEST_UPLOAD:
			ctx, cancel = context.WithCancel(context.Background())
			source.Write(tag, buf)
		default:
			ctx, cancel = context.WithTimeout(context.Background(), time.Second*15)
			source.Write(tag, buf)
		}

		<-ctx.Done()
	}
}

func addDeployCmd(rootCmd *cobra.Command, uploadCmd *cobra.Command, removeCmd *cobra.Command, createCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy src_file[.go|.zip|dir]",
		Short: "Deploy your serverless, this is an alias of chaining commands (upload -> remove -> create)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			uploadCmd.Run(uploadCmd, args)
			if errMsg := lastError(); errMsg != "" {
				os.Exit(1)
			}
			removeCmd.Run(removeCmd, args)
			if errMsg := lastError(); errMsg != "" {
				os.Exit(1)
			}
			createCmd.Run(createCmd, args)
			if errMsg := lastError(); errMsg != "" {
				os.Exit(1)
			}

			fmt.Println("Successfully!")
		},
		GroupID: groupIDGeneral,
	}
	rootCmd.AddCommand(cmd)
	cmd.Flags().StringArrayVar(&envs, "env", nil, "Set environment variables")

	return cmd
}

func Handler(yctx serverless.Context) {
	var res pkg.Response
	err := json.Unmarshal(yctx.Data(), &res)
	if err != nil {
		fmt.Println(err)
		return
	}

	if res.Error != "" {
		fmt.Printf("[%s] ERROR: %s\n", res.MeshZone, res.Error)
		resErr.Store(res.Error)
	} else if res.Msg != "" {
		fmt.Printf("[%s] OK: %s\n", res.MeshZone, res.Msg)
	}

	if res.Done {
		resCount.Add(1)
	}
	count := resCount.Load()
	if count > 0 {
		if yctx.Tag() == pkg.TAG_RESPONSE_UPLOAD || count >= meshNum || lastError() != "" {
			cancel()
		}
	}
}

func initViper() error {
	v := viper.GetViper()
	configFile := "./yc.yml"
	if c, ok := os.LookupEnv("YC_CONFIG_FILE"); ok {
		configFile = c
	}
	v.SetConfigFile(configFile)

	if _, err := os.Stat(configFile); err == nil {
		err = v.ReadInConfig()
		if err != nil {
			return err
		}
	}

	if v.IsSet("zipper") {
		zipperAddr = v.GetString("zipper")
	}

	if v.IsSet("secret") {
		secret = v.GetString("secret")
	}

	if v.IsSet("tool") {
		tool = v.GetString("tool")
	}

	if v.IsSet("mesh") {
		meshNum = v.GetUint32("mesh")
	}

	return nil
}

func main() {
	nanoid, err := gonanoid.New(8)
	if err != nil {
		fmt.Println("generate target error:", err)
		os.Exit(1)
	}
	target = nanoid

	if _, ok := os.LookupEnv("YOMO_LOG_LEVEL"); !ok {
		os.Setenv("YOMO_LOG_OUTPUT", "/dev/null")
		os.Setenv("YOMO_LOG_ERROR_OUTPUT", "/dev/null")
	}

	rootCmd := &cobra.Command{
		Use:   "yc",
		Short: "Manage your globally deployed Serverless LLM Functions on vivgrid.com from the command line",
	}

	rootCmd.PersistentFlags().StringVar(&zipperAddr, "zipper", "zipper.vivgrid.com:9000", "Vivgrid zipper endpoint")
	rootCmd.PersistentFlags().StringVar(&secret, "secret", "", "Vivgrid App secret")
	rootCmd.PersistentFlags().StringVar(&tool, "tool", "my_first_llm_tool", "Serverless LLM Tool name")

	err = initViper()
	if err != nil {
		fmt.Println("init viper error:", err)
		os.Exit(1)
	}

	// Normalize zipperAddr after all configuration sources are processed
	zipperAddr = normalizeZipperAddr(zipperAddr)

	uploadCmd := addUploadCmd(rootCmd)
	removeCmd := addRemoveCmd(rootCmd)
	createCmd := addCreateCmd(rootCmd)

	_ = addVersionCmd(rootCmd)
	_ = addStatusCmd(rootCmd)
	_ = addLogsCmd(rootCmd)
	_ = addDeployCmd(rootCmd, uploadCmd, removeCmd, createCmd)
	addDocCmd(rootCmd)

	rootCmd.AddGroup(&cobra.Group{
		ID:    groupIDGeneral,
		Title: colorBlue + "General" + colorReset,
	})

	rootCmd.AddGroup(&cobra.Group{
		ID:    groupIDDeployment,
		Title: colorBlue + "Manage serverless deployment" + colorReset,
	})

	rootCmd.AddGroup(&cobra.Group{
		ID:    groupIDMonitoring,
		Title: colorBlue + "Observability" + colorReset,
	})

	err = rootCmd.Execute()
	if err != nil {
		fmt.Println("cmd error:", err)
		os.Exit(1)
	}
}

const (
	groupIDDeployment = "deployment"
	groupIDMonitoring = "monitoring"
	groupIDGeneral    = "general"

	colorReset = "\033[0m"
	colorBlue  = "\033[34m"
)
