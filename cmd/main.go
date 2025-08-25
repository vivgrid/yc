package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	meshNum    uint32
	resCount   atomic.Uint32
	cancel     context.CancelFunc
)

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

func execProcess(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = ""
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func addVersionCmd(rootCmd *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("yc version:", pkg.CliVersion)
		},
	}
	rootCmd.AddCommand(cmd)
}

func addUploadCmd(rootCmd *cobra.Command) {
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
	}
	rootCmd.AddCommand(cmd)
	cmd.GroupID = groupIDGeneral
}

func addCreateCmd(rootCmd *cobra.Command) {
	var envs []string
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
	}
	rootCmd.AddCommand(cmd)
	cmd.Flags().StringArrayVar(&envs, "env", nil, "Set environment variable")
	cmd.GroupID = groupIDDeployment
}

func addRemoveCmd(rootCmd *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Delete current serverless deployment",
		Args:  cobra.ExactArgs(0),
		Run: run(
			pkg.TAG_REQUEST_REMOVE,
			&pkg.ReqMsgRemove{},
			nil,
		),
	}
	rootCmd.AddCommand(cmd)
	cmd.GroupID = groupIDDeployment
}

func addStatusCmd(rootCmd *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show serverless status",
		Args:  cobra.ExactArgs(0),
		Run: run(
			pkg.TAG_REQUEST_STATUS,
			&pkg.ReqMsgStatus{},
			nil,
		),
	}
	rootCmd.AddCommand(cmd)
	cmd.GroupID = groupIDMonitoring
}

func addLogsCmd(rootCmd *cobra.Command) {
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
	}
	rootCmd.AddCommand(cmd)
	cmd.Flags().IntVar(&tail, "tail", 20, "Tail logs")
	cmd.GroupID = groupIDMonitoring
}

func run[T any](tag uint32, reqMsg *T, f func([]string) error) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
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
			err = f(cmd.Flags().Args())
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

func addDeployCmd(rootCmd *cobra.Command) {
	var envs []string
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy your serverless, this is an alias of chaining commands (upload -> remove -> create)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			yc := os.Args[0]

			src := args[0]
			err := execProcess(yc, "upload", src)
			if err != nil {
				fmt.Println("yc upload error:", err)
				os.Exit(1)
			}

			err = execProcess(yc, "remove")
			if err != nil {
				fmt.Println("yc remove error:", err)
				os.Exit(1)
			}

			createArgs := []string{"create"}
			for _, env := range envs {
				createArgs = append(createArgs, "--env")
				createArgs = append(createArgs, env)
			}
			err = execProcess(yc, createArgs...)
			if err != nil {
				fmt.Println("yc create error:", err)
				os.Exit(1)
			}
			fmt.Println("Successfully!")
		},
	}
	rootCmd.AddCommand(cmd)
	cmd.Flags().StringArrayVar(&envs, "env", nil, "Set environment variables")
	cmd.GroupID = groupIDGeneral
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
	} else if res.Msg != "" {
		fmt.Printf("[%s] OK: %s\n", res.MeshZone, res.Msg)
	}

	if res.Done {
		resCount.Add(1)
	}
	count := resCount.Load()
	if count > 0 {
		if yctx.Tag() == pkg.TAG_RESPONSE_UPLOAD || count >= meshNum {
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

	meshNum = 7

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
	// rootCmd.PersistentFlags().Uint32Var(&meshNum, "mesh-num", 7, "Number of mesh nodes")

	err = initViper()
	if err != nil {
		fmt.Println("init viper error:", err)
		os.Exit(1)
	}

	// Normalize zipperAddr after all configuration sources are processed
	zipperAddr = normalizeZipperAddr(zipperAddr)

	addVersionCmd(rootCmd)
	addUploadCmd(rootCmd)
	addCreateCmd(rootCmd)
	addRemoveCmd(rootCmd)
	addStatusCmd(rootCmd)
	addLogsCmd(rootCmd)
	addDeployCmd(rootCmd)

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
	groupIDState      = "state"
	groupIDMonitoring = "monitoring"
	groupIDGeneral    = "general"

	colorReset = "\033[0m"
	colorBlue  = "\033[34m"
)
