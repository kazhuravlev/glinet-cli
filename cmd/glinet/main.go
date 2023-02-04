package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/imroc/req/v3"
	cli "github.com/urfave/cli/v3"
	"log"
	"os"
	"path/filepath"
)

const envPassword = "GL_INET_PASSWORD"
const cfgFile = ".config/glinet/auth-token"
const argAddr = "addr"
const argPassword = "password"

func main() {
	app := &cli.App{ //nolint:exhaustruct
		Name: "glinet",
		Commands: []*cli.Command{
			{
				Name:        "auth",
				Description: "Auth in router",
				Action:      cmdAuth,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     argAddr,
						Aliases:  []string{"a"},
						Required: false,
						Value:    "192.168.8.1",
					},
					&cli.StringFlag{
						Name:     argPassword,
						Aliases:  []string{"p"},
						Required: false,
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func cmdAuth(c *cli.Context) error {
	ctx := c.Context
	fmt.Println(c.String(argAddr), c.String(argPassword))
	if c.NumFlags() < 2 {
		return errors.New("specify address and password from router")
	}

	token, err := fetchToken(ctx, c.String(argAddr), c.String(argPassword))
	if err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	absCfgFile := filepath.Join(homeDir, cfgFile)
	_ = os.MkdirAll(filepath.Dir(absCfgFile), 0o755)

	if err := os.WriteFile(absCfgFile, []byte(token), 0o644); err != nil {
		return err
	}

	return nil
}

func fetchToken(ctx context.Context, addr string, password string) (string, error) {
	client := req.C()
	resp, err := client.R().
		SetContext(ctx).
		SetFormData(map[string]string{"pwd": password}).
		Post(fmt.Sprintf("http://%s/api/router/login", addr))
	if err != nil {
		return "", err
	}

	var m struct {
		Token string `json:"token"`
	}
	if err := resp.UnmarshalJson(&m); err != nil {
		return "", err
	}

	return m.Token, nil
}
