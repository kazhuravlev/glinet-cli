package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/imroc/req/v3"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/kazhuravlev/just"
	cli "github.com/urfave/cli/v3"
	"golang.org/x/term"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const envPassword = "GL_INET_PASSWORD"
const cfgFile = ".config/glinet/config.json"

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
	just.SliceSort(res.Clients, func(a, b Client) bool {
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

func cmdGetModemInfo(ctx context.Context, c *cli.Context, client *req.Client) error {
	resp, err := client.R().
		SetContext(ctx).
		Post("/cgi-bin/api/modem/info")
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return errors.New("unexpected status code")
	}

	type Modem struct {
		Ports       []string `json:"ports"`
		ModemID     int      `json:"modem_id"`
		DataPort    string   `json:"data_port"`
		ControlPort string   `json:"control_port"`
		QmiPort     string   `json:"qmi_port"`
		Name        string   `json:"name"`
		Imei        string   `json:"IMEI"`
		Bus         string   `json:"bus"`
		HwVersion   string   `json:"hw_version"`
		SimNum      string   `json:"sim_num"`
		Mnc         string   `json:"mnc"`
		Mcc         string   `json:"mcc"`
		Carrier     string   `json:"carrier"`
		Up          string   `json:"up"`
		SIMStatus   int      `json:"SIM_status"`
		Operators   []string `json:"operators"`
	}

	var res struct {
		Passthrough           bool    `json:"passthrough"`
		HintModifyWifiChannel int     `json:"hint_modify_wifi_channel"`
		Modems                []Modem `json:"modems"`
	}
	if err := resp.UnmarshalJson(&res); err != nil {
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

func cmdTurnModemOn(ctx context.Context, c *cli.Context, client *req.Client) error {
	request := map[string]string{
		//"modem_id": "1",
		//"bus":      "1-1.2",
		"disable": "false",
	}

	client.DevMode()
	resp, err := client.R().
		SetContext(ctx).
		SetFormData(request).
		Post("/cgi-bin/api/modem/enable")
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return errors.New("unexpected status code")
	}

	return nil
}

func cmdTurnModemOff(ctx context.Context, c *cli.Context, client *req.Client) error {
	request := map[string]string{
		//"modem_id": "1",
		//"bus":      "1-1.2",
		"disable": "true",
	}

	client.DevMode()
	resp, err := client.R().
		SetContext(ctx).
		SetFormData(request).
		Post("/cgi-bin/api/modem/enable")
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return errors.New("unexpected status code")
	}

	return nil
}

func cmdTurnModemOnAuto(ctx context.Context, c *cli.Context, client *req.Client) error {
	request := map[string]string{
		"modem_id": "1",
		"bus":      "1-1.2",
	}

	client.DevMode()
	resp, err := client.R().
		SetContext(ctx).
		SetFormData(request).
		Post("/cgi-bin/api/modem/auto")
	if err != nil {
		return err
	}

	if resp.GetStatusCode() != http.StatusOK {
		return errors.New("unexpected status code")
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

	glToken, err := fetchToken(ctx, glAddr, glPassword)
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
	cfg.Routers = append(cfg.Routers, Router{
		Addr:     glAddr,
		Password: glPassword,
		Token:    glToken,
	})
	bb, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(absCfgFile, bb, 0o644); err != nil {
		return err
	}

	fmt.Println("Authorization successful")

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

		if cfg.Version != VersionV1 {
			return errors.New("unsupported config version")
		}

		if len(cfg.Routers) != 1 {
			return errors.New("not implemented")
		}

		glAddr := cfg.Routers[0].Addr
		glToken := cfg.Routers[0].Token

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
