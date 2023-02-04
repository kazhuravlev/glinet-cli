package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/imroc/req/v3"
	"github.com/kazhuravlev/just"
	cli "github.com/urfave/cli/v3"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const envPassword = "GL_INET_PASSWORD"
const cfgFile = ".config/glinet/config.json"
const argAddr = "addr"
const argPassword = "password"

type Config struct {
	Tokens map[string]string `json:"tokens"`
}

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
			{
				Name:        "public-ip",
				Description: "Get public IP addr",
				Action:      wrapWithClient(cmdGetPublicIP),
			},
			{
				Name:        "check-internet",
				Description: "Check that internet is reachable",
				Action:      wrapWithClient(cmdGetInetReachable),
			},
			{
				Name:        "clients-list",
				Description: "Get list of clients",
				Action:      wrapWithClient(cmdGetClients),
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func cmdGetPublicIP(ctx context.Context, c *cli.Context, client *req.Client) error {
	resp, err := client.R().
		SetContext(ctx).
		Get("/cgi-bin/api/internet/public_ip/get")
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return errors.New("unexpected status code")
	}

	var res struct {
		ServerIP string `json:"serverip"`
	}
	if err := resp.UnmarshalJson(&res); err != nil {
		return err
	}

	fmt.Println("server IP", res.ServerIP)
	return nil
}

func cmdGetInetReachable(ctx context.Context, c *cli.Context, client *req.Client) error {
	resp, err := client.R().
		SetContext(ctx).
		Get("/cgi-bin/api/internet/reachable")
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return errors.New("unexpected status code")
	}

	var res struct {
		Reachable  bool `json:"reachable"`
		RebootFlag bool `json:"reboot_flag"`
	}
	if err := resp.UnmarshalJson(&res); err != nil {
		return err
	}

	fmt.Println(res)
	return nil
}

func cmdGetClients(ctx context.Context, c *cli.Context, client *req.Client) error {
	resp, err := client.R().
		SetContext(ctx).
		Get("/cgi-bin/api/client/list")
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return errors.New("unexpected status code")
	}

	type Client struct {
		Remote     bool   `json:"remote"`
		Mac        string `json:"mac"`
		Favorite   bool   `json:"favorite"`
		IP         string `json:"ip"`
		Up         string `json:"up"`
		Down       string `json:"down"`
		TotalUp    string `json:"total_up"`
		TotalDown  string `json:"total_down"`
		QosUp      string `json:"qos_up"`
		QosDown    string `json:"qos_down"`
		Blocked    bool   `json:"blocked"`
		Iface      string `json:"iface"`
		Name       string `json:"name"`
		OnlineTime string `json:"online_time"`
		Alive      string `json:"alive"`
		NewOnline  bool   `json:"new_online"`
		Online     bool   `json:"online"`
		Vendor     string `json:"vendor"`
		Node       string `json:"node"`
	}

	var res struct {
		Clients []Client `json:"clients"`
	}
	if err := resp.UnmarshalJson(&res); err != nil {
		return err
	}

	for _, glClient := range res.Clients {
		fmt.Println(glClient.Iface, glClient.IP, glClient.Online, glClient.Name)
	}
	
	return nil
}

func getAbsConfigFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, cfgFile), nil
}

func cmdAuth(c *cli.Context) error {
	ctx := c.Context
	fmt.Println(c.String(argAddr), c.String(argPassword))
	if c.NumFlags() < 2 {
		return errors.New("specify address and password from router")
	}

	glAddr := c.String(argAddr)
	glToken, err := fetchToken(ctx, glAddr, c.String(argPassword))
	if err != nil {
		return err
	}

	absCfgFile, err := getAbsConfigFile()
	if err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(absCfgFile), 0o755)

	cfg := Config{Tokens: make(map[string]string)}
	if bb, err := os.ReadFile(absCfgFile); err == nil {
		if err := json.Unmarshal(bb, &cfg); err != nil {
			return fmt.Errorf("config file is corrupted. check cfg file or delete it: %w", err)
		}
	}

	cfg.Tokens[glAddr] = glToken
	bb, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(absCfgFile, bb, 0o644); err != nil {
		return err
	}

	return nil
}

func wrapWithClient(cmd func(context.Context, *cli.Context, *req.Client) error) cli.ActionFunc {
	// FIXME: support several routers

	return func(c *cli.Context) error {
		absConfigFile, err := getAbsConfigFile()
		if err != nil {
			return err
		}

		bb, err := os.ReadFile(absConfigFile)
		if err != nil {
			return err
		}

		var cfg Config
		if err := json.Unmarshal(bb, &cfg); err != nil {
			return err
		}

		if len(cfg.Tokens) != 1 {
			return errors.New("not implemented")
		}

		glAddr := just.MapPairs(cfg.Tokens)[0].Key
		glToken := just.MapPairs(cfg.Tokens)[0].Val

		client := req.C().
			SetBaseURL(fmt.Sprintf("https://%s", glAddr)).
			SetCommonHeader("Authorization", glToken).
			EnableInsecureSkipVerify()

		// TODO: add timeouts
		ctx := c.Context
		return cmd(ctx, c, client)
	}
}

func fetchToken(ctx context.Context, addr string, password string) (string, error) {
	client := req.C().EnableInsecureSkipVerify()
	resp, err := client.R().
		SetContext(ctx).
		SetFormData(map[string]string{"pwd": password}).
		Post(fmt.Sprintf("https://%s/api/router/login", addr))
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
