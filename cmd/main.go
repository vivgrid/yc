package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"sync/atomic"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/u-root/u-root/pkg/uzip"
	"github.com/yomorun/yomo"
	"github.com/yomorun/yomo/serverless"

	"github.com/yomorun/yc-cli/pkg"
)

var (
	target     string
	zipperAddr string
	appKey     string
	appSecret  string
	sfnName    string
	meshNum    uint32
	resCount   atomic.Uint32
	cancel     context.CancelFunc
)

func addUploadCmd(rootCmd *cobra.Command) {
	msg := &pkg.ReqMsgUpload{}
	var src string
	cmd := subCmd(rootCmd, pkg.TAG_UPLOAD, msg,
		func() error {
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

				err = uzip.ToZip(src, zipPath, "")
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
	)
	cmd.Flags().StringVar(&src, "src", "", "the path of src code [.go | .zip | dir]")
	cmd.MarkFlagRequired("src")
	bindFlags(cmd.Flags())
}

func addCreateCmd(rootCmd *cobra.Command) {
	var envs []string
	cmd := subCmd(rootCmd, pkg.TAG_CREATE, &pkg.ReqMsgCreate{
		Envs: &envs,
	}, nil)
	cmd.Flags().StringArrayVar(&envs, "envs", nil, "app environment variables")
	bindFlags(cmd.Flags())
}

func addStopCmd(rootCmd *cobra.Command) {
	var timeout int
	cmd := subCmd(rootCmd, pkg.TAG_STOP, &pkg.ReqMsgStop{
		Timeout: &timeout,
	}, nil)
	cmd.Flags().IntVar(&timeout, "timeout", 10, "timeout")
	bindFlags(cmd.Flags())
}

func addStartCmd(rootCmd *cobra.Command) {
	subCmd(rootCmd, pkg.TAG_START, &pkg.ReqMsgStart{}, nil)
}

func addRemoveCmd(rootCmd *cobra.Command) {
	subCmd(rootCmd, pkg.TAG_REMOVE, &pkg.ReqMsgRemove{}, nil)
}

func addStatusCmd(rootCmd *cobra.Command) {
	subCmd(rootCmd, pkg.TAG_STATUS, &pkg.ReqMsgStatus{}, nil)
}

func addLogsCmd(rootCmd *cobra.Command) {
	var tail int
	cmd := subCmd(rootCmd, pkg.TAG_LOGS, &pkg.ReqMsgLogs{
		Tail: &tail,
	}, nil)
	cmd.Flags().IntVar(&tail, "tail", 20, "tail")
	bindFlags(cmd.Flags())
}

func subCmd[T any](rootCmd *cobra.Command, tag uint32, msg *T, f func() error) *cobra.Command {
	cmd := &cobra.Command{
		Use: pkg.TagName(tag),
		Run: func(cmd *cobra.Command, args []string) {
			credential := "app-key-secret:" + appKey + "|" + appSecret

			sfn := yomo.NewStreamFunction(
				"yc-response", zipperAddr, yomo.WithSfnCredential(credential))
			sfn.SetHandler(Handler)
			sfn.SetObserveDataTags(pkg.ResponseTag(tag))
			sfn.SetWantedTarget(target)
			err := sfn.Connect()
			if err != nil {
				log.Fatalln(err)
			}
			defer sfn.Close()

			source := yomo.NewSource(
				"yc-request", zipperAddr, yomo.WithCredential(credential))
			err = source.Connect()
			if err != nil {
				log.Fatalln(err)
			}
			defer source.Close()

			if f != nil {
				err = f()
				if err != nil {
					log.Fatalln(err)
				}
			}

			req := &pkg.Request[T]{
				Version: pkg.Version,
				Target:  target,
				SfnName: sfnName,
				Msg:     msg,
			}

			buf, _ := json.Marshal(req)

			var ctx context.Context
			switch tag {
			case pkg.TAG_LOGS:
				ctx, cancel = context.WithCancel(context.Background())
				go func() {
					for {
						source.Write(tag, buf)
						time.Sleep(time.Second * 15)
					}
				}()
			case pkg.TAG_UPLOAD:
				ctx, cancel = context.WithCancel(context.Background())
				source.Write(tag, buf)
			default:
				ctx, cancel = context.WithTimeout(context.Background(), time.Second*15)
				source.Write(tag, buf)
			}

			<-ctx.Done()
		},
	}
	rootCmd.AddCommand(cmd)
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
		fmt.Printf("[%s.%s] ERROR: %s\n", res.MeshZone, res.MeshNode, res.Error)
	} else if res.Msg != "" {
		fmt.Printf("[%s.%s] OK: %s\n", res.MeshZone, res.MeshNode, res.Msg)
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
	v.SetConfigName("yc")
	v.SetConfigType("yml")
	v.AddConfigPath(".")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return err
		}
	}
	return nil
}

func bindFlags(fs *pflag.FlagSet) {
	v := viper.GetViper()
	fs.VisitAll(func(f *pflag.Flag) {
		configName := f.Name
		if !f.Changed && v.IsSet(configName) {
			val := v.Get(configName)
			fs.Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

func main() {
	nanoid, err := gonanoid.New(8)
	if err != nil {
		log.Fatalln(err)
	}
	target = nanoid

	if _, ok := os.LookupEnv("YOMO_LOG_LEVEL"); !ok {
		os.Setenv("YOMO_LOG_LEVEL", "error")
	}

	err = initViper()
	if err != nil {
		log.Fatalln(err)
	}

	rootCmd := &cobra.Command{Use: "yc"}

	rootCmd.PersistentFlags().StringVar(&zipperAddr, "zipper", "zipper.allegrocloud.io:9000", "allegro zipper address")
	rootCmd.PersistentFlags().StringVar(&appKey, "app-key", "", "allegro app key")
	rootCmd.PersistentFlags().StringVar(&appSecret, "app-secret", "", "allegro app secret")
	rootCmd.PersistentFlags().Uint32Var(&meshNum, "mesh-num", 7, "mesh zone number")
	rootCmd.PersistentFlags().StringVar(&sfnName, "sfn-name", "", "sfn name")
	rootCmd.MarkPersistentFlagRequired("app-key")
	rootCmd.MarkPersistentFlagRequired("app-secret")
	rootCmd.MarkPersistentFlagRequired("sfn-name")
	bindFlags(rootCmd.PersistentFlags())

	addUploadCmd(rootCmd)
	addCreateCmd(rootCmd)
	addStopCmd(rootCmd)
	addStartCmd(rootCmd)
	addRemoveCmd(rootCmd)
	addStatusCmd(rootCmd)
	addLogsCmd(rootCmd)

	err = rootCmd.Execute()
	if err != nil {
		fmt.Println("cmd error:", err)
		os.Exit(1)
	}
}
