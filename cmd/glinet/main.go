package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/kazhuravlev/glinet-sdk"
	"github.com/kazhuravlev/just"
	cli "github.com/urfave/cli/v3"
	"golang.org/x/term"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const envPassword = "GL_INET_PASSWORD"
const cfgFile = ".config/glinet/config.json"

type Code int

const (
	CodeBadToken Code = -1
)

type Version string

const VersionV1 Version = "v1"

type Router struct {
	Addr     string `json:"addr"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

type Config struct {
	Version Version  `json:"v"`
	Routers []Router `json:"routers"`
}

func main() {
	app := &cli.App{ //nolint:exhaustruct
		Name: "glinet",
		Commands: []*cli.Command{
			{
				Name:        "auth",
				Description: "Auth in router",
				Action:      cmdAuth,
				Usage:       "glinet auth 192.168.8.1 or just glinet auth",
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
			{
				Name:        "get-modem-info",
				Description: "Get status of modem",
				Action:      wrapWithClient(cmdGetModemInfo),
			},
			{
				Name:        "modem-turn-on",
				Description: "Turn on modem",
				Action:      wrapWithClient(cmdTurnModemOn),
			},
			{
				Name:        "modem-turn-off",
				Description: "Turn off modem",
				Action:      wrapWithClient(cmdTurnModemOff),
			},
			{
				Name:        "modem-turn-on-auto",
				Description: "Auto dial",
				Action:      wrapWithClient(cmdTurnModemOnAuto),
			},
			{
				Name:        "modem-restart",
				Description: "Restart modem",
				Action:      wrapWithClient(cmdRestartModem),
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func cmdGetPublicIP(ctx context.Context, c *cli.Context, client *glinet.Client) error {
	ip, err := client.GetPublicIP(ctx)
	if err != nil {
		return err
	}

	fmt.Println("server IP", ip)
	return nil
}

func cmdGetInetReachable(ctx context.Context, c *cli.Context, client *glinet.Client) error {
	res, err := client.GetNetworkStatus(ctx)
	if err != nil {
		return err
	}

	fmt.Println(res)
	return nil
}

func cmdGetClients(ctx context.Context, c *cli.Context, client *glinet.Client) error {
	res, err := client.GetClientList(ctx)
	if err != nil {
		return err
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	t.AppendHeader(table.Row{
		"IP",
		"Mac",
		"Online",
		"Iface",
		"Name",
		"Favorite",
		"Blocked",
		"OnlineTime",
		"Alive",
	})
	just.SliceSort(res.Clients, func(a, b glinet.RouterClient) bool {
		return a.Online != b.Online
	})
	for _, glClient := range res.Clients {
		t.AppendRow(table.Row{
			glClient.IP,
			glClient.Mac,
			glClient.Online,
			glClient.Iface,
			glClient.Name,
			glClient.Favorite,
			glClient.Blocked,
			glClient.OnlineTime,
			glClient.Alive,
		})

	}

	t.Render()
	return nil
}

func cmdGetModemInfo(ctx context.Context, c *cli.Context, client *glinet.Client) error {
	res, err := client.GetModemInfo(ctx)
	if err != nil {
		return err
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetColumnConfigs([]table.ColumnConfig{
		{
			Number:   1,
			WidthMax: 12,
		},
		{},
	})

	for i, modem := range res.Modems {
		t.AppendRow(table.Row{fmt.Sprintf("#%d", i+1)}, table.RowConfig{
			AutoMerge:      true,
			AutoMergeAlign: text.AlignLeft,
		})
		t.AppendSeparator()
		t.AppendRows([]table.Row{
			{"ModemID", modem.ModemID},
			{"Name", modem.Name},
			{"Imei", modem.Imei},
			{"Carrier", modem.Carrier},
			{"Up", modem.Up},
			{"SIMStatus", modem.SIMStatus},
			{"Ports", strings.Join(modem.Ports, ", ")},
			{"DataPort", modem.DataPort},
			{"ControlPort", modem.ControlPort},
			{"QmiPort", modem.QmiPort},
			{"Bus", modem.Bus},
			{"HwVersion", modem.HwVersion},
			{"SimNum", modem.SimNum},
			{"Mnc", modem.Mnc},
			{"Mcc", modem.Mcc},
			{"Operators", strings.Join(modem.Operators, ", ")},
		})

		fmt.Println(modem.SIMStatus, modem.Up, modem.Imei, modem.Carrier, modem.QmiPort)
	}

	t.Render()

	return nil
}

func cmdTurnModemOn(ctx context.Context, c *cli.Context, client *glinet.Client) error {
	return client.ModemTurnOn(ctx)
}

func cmdTurnModemOff(ctx context.Context, c *cli.Context, client *glinet.Client) error {
	return client.ModemTurnOff(ctx)
}

func cmdTurnModemOnAuto(ctx context.Context, c *cli.Context, client *glinet.Client) error {
	return client.ModemTurnOnAuto(ctx)
}

func cmdRestartModem(ctx context.Context, c *cli.Context, client *glinet.Client) error {
	if err := client.ModemTurnOff(ctx); err != nil {
		return err
	}

	return client.ModemTurnOnAuto(ctx)
}

func cmdAuth(c *cli.Context) error {
	ctx := c.Context

	// NOTE: this is default address for all of Gl.Inet devices
	address := "192.168.8.1"
	if c.Args().Len() == 1 {
		address = c.Args().First()
	}

	ip := net.ParseIP(address)
	if ip == nil {
		return errors.New("unable to parse ip")
	}

	glAddr := ip.String()
	fmt.Printf("Address: '%s'\n", glAddr)
	fmt.Print("Enter Password: ")
	bytePassword, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return err
	}

	fmt.Println()

	glPassword := strings.TrimSpace(string(bytePassword))

	glClient, err := glinet.NewFromPassword(ctx, glAddr, glPassword)
	if err != nil {
		return err
	}

	absCfgFile, err := getAbsConfigFile()
	if err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(absCfgFile), 0o755)

	var cfg Config
	if bb, err := os.ReadFile(absCfgFile); err == nil {
		if err := json.Unmarshal(bb, &cfg); err != nil {
			return fmt.Errorf("config file is corrupted. check cfg file or delete it: %w", err)
		}
	}

	cfg.Version = VersionV1
	newRouter := Router{
		Addr:     glAddr,
		Password: glPassword,
		Token:    glClient.Token(),
	}
	cfg.Routers = just.SliceReplaceFirstOrAdd(
		cfg.Routers,
		func(_ int, router Router) bool { return router.Addr == glAddr },
		newRouter,
	)

	bb, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(absCfgFile, bb, 0o644); err != nil {
		return err
	}

	fmt.Println("Authorization successful")

	return nil
}

func getAbsConfigFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, cfgFile), nil
}

func wrapWithClient(cmd func(context.Context, *cli.Context, *glinet.Client) error) cli.ActionFunc {
	// FIXME: support several routers

	return func(c *cli.Context) error {
		router, err := parseCredentials()
		if err != nil {
			return err
		}

		glClient, err := glinet.New(router.Addr, router.Token)
		if err != nil {
			return err
		}

		// TODO: add timeouts
		ctx := c.Context

		return cmd(ctx, c, glClient)
	}
}

func parseCredentials() (*Router, error) {
	absConfigFile, err := getAbsConfigFile()
	if err != nil {
		return nil, err
	}

	bb, err := os.ReadFile(absConfigFile)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(bb, &cfg); err != nil {
		return nil, err
	}

	if cfg.Version != VersionV1 {
		return nil, errors.New("unsupported config version")
	}

	if len(cfg.Routers) != 1 {
		return nil, errors.New("not implemented")
	}

	return &cfg.Routers[0], nil
}
