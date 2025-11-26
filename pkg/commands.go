package pkg

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

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yomorun/yomo"
	"github.com/yomorun/yomo/serverless"
)

type command struct {
	tid        string
	zipperAddr string
	secret     string
	tool       string
	meshNum    uint32
	resCount   atomic.Uint32
	resErr     atomic.Value
	cancel     context.CancelFunc
	envs       []string
}

func Execute(rootCmd *cobra.Command, configFile string, tid string, defaultZipperAddr string, defaultMeshNum uint32) error {
	if _, ok := os.LookupEnv("YOMO_LOG_LEVEL"); !ok {
		os.Setenv("YOMO_LOG_OUTPUT", "/dev/null")
		os.Setenv("YOMO_LOG_ERROR_OUTPUT", "/dev/null")
	}

	c := &command{
		tid:     tid,
		meshNum: defaultMeshNum,
	}

	rootCmd.PersistentFlags().StringVar(&c.zipperAddr, "zipper", defaultZipperAddr, "zipper endpoint")
	rootCmd.PersistentFlags().StringVar(&c.secret, "secret", "", "app secret")
	rootCmd.PersistentFlags().StringVar(&c.tool, "tool", "my_first_llm_tool", "serverless LLM tool name")

	uploadCmd := c.addUploadCmd(rootCmd)
	removeCmd := c.addRemoveCmd(rootCmd)
	createCmd := c.addCreateCmd(rootCmd)

	c.addVersionCmd(rootCmd)
	c.addStatusCmd(rootCmd)
	c.addLogsCmd(rootCmd)
	c.addDeployCmd(rootCmd, uploadCmd, removeCmd, createCmd)
	c.addDocCmd(rootCmd)

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

	if configFile != "" {
		v := viper.GetViper()
		v.SetConfigFile(configFile)

		err := v.ReadInConfig()
		if err != nil {
			return err
		}

		if v.IsSet("zipper") {
			c.zipperAddr = v.GetString("zipper")
		}

		if v.IsSet("secret") {
			c.secret = v.GetString("secret")
		}

		if v.IsSet("tool") {
			c.tool = v.GetString("tool")
		}

		if v.IsSet("mesh") {
			c.meshNum = v.GetUint32("mesh")
		}
	}

	// Normalize zipperAddr after all configuration sources are processed
	c.zipperAddr = normalizeZipperAddr(c.zipperAddr)

	return rootCmd.Execute()
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

// lastError retrieves the latest error message recorded by the handler.
func (c *command) lastError() string {
	if v := c.resErr.Load(); v != nil {
		if msg, ok := v.(string); ok {
			return msg
		}
	}
	return ""
}

// addDocCmd adds the documentation command to the root command
func (c *command) addDocCmd(rootCmd *cobra.Command) {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.DisableAutoGenTag = true
	cmd := &cobra.Command{
		Use:    "doc",
		Short:  "Generate documentation for the CLI commands",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return GenDoc(rootCmd)
		},
	}
	rootCmd.AddCommand(cmd)
}

func (c *command) addVersionCmd(rootCmd *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version",
		Args:  cobra.ExactArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("version:", CliVersion)
		},
	}
	rootCmd.AddCommand(cmd)
}

func (c *command) addUploadCmd(rootCmd *cobra.Command) *cobra.Command {
	msg := &ReqMsgUpload{}
	cmd := &cobra.Command{
		Use:   "upload src_file[.go|.zip|dir]",
		Short: "Upload the source code and compile",
		Args:  cobra.ExactArgs(1),
		Run: run(
			c,
			TAG_REQUEST_UPLOAD,
			msg,
			func(args []string) error {
				src := args[0]
				info, err := os.Stat(src)
				if err != nil {
					return err
				}

				var data []byte

				if info.IsDir() {
					f, err := os.CreateTemp("", "app-*.zip")
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

func (c *command) addCreateCmd(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create serverless deployment and start it",
		Args:  cobra.ExactArgs(0),
		Run: run(
			c,
			TAG_REQUEST_CREATE,
			&ReqMsgCreate{
				Envs: &c.envs,
			},
			nil,
		),
		GroupID: groupIDDeployment,
	}
	rootCmd.AddCommand(cmd)
	cmd.Flags().StringArrayVar(&c.envs, "env", nil, "Set environment variable")
	return cmd
}

func (c *command) addRemoveCmd(rootCmd *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Delete current serverless deployment",
		Args:  cobra.ExactArgs(0),
		Run: run(
			c,
			TAG_REQUEST_REMOVE,
			&ReqMsgRemove{},
			nil,
		),
		GroupID: groupIDDeployment,
	}
	rootCmd.AddCommand(cmd)

	return cmd
}

func (c *command) addStatusCmd(rootCmd *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show serverless status",
		Args:  cobra.ExactArgs(0),
		Run: run(
			c,
			TAG_REQUEST_STATUS,
			&ReqMsgStatus{},
			nil,
		),
		GroupID: groupIDMonitoring,
	}
	rootCmd.AddCommand(cmd)
}

func (c *command) addLogsCmd(rootCmd *cobra.Command) {
	var tail int
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Observe serverless logs in real-time",
		Args:  cobra.ExactArgs(0),
		Run: run(
			c,
			TAG_REQUEST_LOGS,
			&ReqMsgLogs{},
			nil,
		),
		GroupID: groupIDMonitoring,
	}
	rootCmd.AddCommand(cmd)
	cmd.Flags().IntVar(&tail, "tail", 20, "Tail logs")
}

func (c *command) addDeployCmd(rootCmd *cobra.Command, uploadCmd *cobra.Command, removeCmd *cobra.Command, createCmd *cobra.Command) {
	cmd := &cobra.Command{
		Use:   "deploy src_file[.go|.zip|dir]",
		Short: "Deploy your serverless, this is an alias of chaining commands (upload -> remove -> create)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			uploadCmd.Run(uploadCmd, args)
			if errMsg := c.lastError(); errMsg != "" {
				os.Exit(1)
			}
			removeCmd.Run(removeCmd, args)
			if errMsg := c.lastError(); errMsg != "" {
				os.Exit(1)
			}
			createCmd.Run(createCmd, args)
			if errMsg := c.lastError(); errMsg != "" {
				os.Exit(1)
			}

			fmt.Println("Successfully!")
		},
		GroupID: groupIDGeneral,
	}
	rootCmd.AddCommand(cmd)
	cmd.Flags().StringArrayVar(&c.envs, "env", nil, "Set environment variables")
}

func run[T any](c *command, tag uint32, reqMsg *T, f func([]string) error) func(cmd *cobra.Command, args []string) {
	return func(cmd *cobra.Command, args []string) {
		sfn := yomo.NewStreamFunction("res:"+c.tid, c.zipperAddr, yomo.WithSfnCredential(c.secret))
		sfn.SetHandler(c.handler)
		sfn.SetObserveDataTags(ResponseTag(tag))
		sfn.SetWantedTarget(c.tid)
		err := sfn.Connect()
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		defer sfn.Close()

		source := yomo.NewSource("req:"+c.tid, c.zipperAddr, yomo.WithCredential(c.secret))
		err = source.Connect()
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}
		defer source.Close()

		if f != nil {
			err = f(args)
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
		}

		req := &Request[T]{
			Version: SpecVersion,
			Target:  c.tid,
			SfnName: c.tool,
			Msg:     reqMsg,
		}

		buf, _ := json.Marshal(req)

		var ctx context.Context
		switch tag {
		case TAG_REQUEST_LOGS:
			ctx, c.cancel = context.WithCancel(context.Background())
			go func() {
				for {
					source.Write(tag, buf)
					time.Sleep(time.Second * 15)
				}
			}()
		case TAG_REQUEST_UPLOAD:
			ctx, c.cancel = context.WithCancel(context.Background())
			source.Write(tag, buf)
		default:
			ctx, c.cancel = context.WithTimeout(context.Background(), time.Second*15)
			source.Write(tag, buf)
		}

		<-ctx.Done()
	}
}

func (c *command) handler(yctx serverless.Context) {
	var res Response
	err := json.Unmarshal(yctx.Data(), &res)
	if err != nil {
		fmt.Println(err)
		return
	}

	if res.Error != "" {
		fmt.Printf("[%s] Error: %s\n", res.MeshZone, res.Error)
		c.resErr.Store(res.Error)
	} else if res.Msg != "" {
		fmt.Printf("[%s] OK: %s\n", res.MeshZone, res.Msg)
	}

	if res.Done {
		c.resCount.Add(1)
	}
	count := c.resCount.Load()
	if count > 0 {
		if yctx.Tag() == TAG_RESPONSE_UPLOAD || count >= c.meshNum || c.lastError() != "" {
			c.cancel()
		}
	}
}

const (
	groupIDDeployment = "deployment"
	groupIDMonitoring = "monitoring"
	groupIDGeneral    = "general"

	colorReset = "\033[0m"
	colorBlue  = "\033[34m"
)
