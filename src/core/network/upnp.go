package network

// --- UPnP --- //

import (
	"errors"
	"github.com/huin/goupnp/dcps/internetgateway2"
	"github.com/robfig/cron/v3"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"log"
)

type RouterClient interface {
	AddPortMapping(
		NewRemoteHost string,
		NewExternalPort uint16,
		NewProtocol string,
		NewInternalPort uint16,
		NewInternalClient string,
		NewEnabled bool,
		NewPortMappingDescription string,
		NewLeaseDuration uint32,
	) (err error)
	GetExternalIPAddress() (
		NewExternalIPAddress string,
		err error,
	)
}

func OpenApplicationPort(port uint16) {
	crontab := cron.New(cron.WithSeconds())
	_, err := crontab.AddFunc("@every 59m", func() {
		upnpForwardPort(context.Background(), port)
	})
	if err != nil {
		panic("Could not start pwnnat server requests")
	}
	crontab.Start()
}

func pickRouterClient(ctx context.Context) (RouterClient, error) {
	tasks, _ := errgroup.WithContext(ctx)
	// Request each client in parallel, and return what is found
	var ip1Clients []*internetgateway2.WANIPConnection1
	tasks.Go(func() error {
		var err error
		ip1Clients, _, err = internetgateway2.NewWANIPConnection1Clients()
		return err
	})
	var ip2Clients []*internetgateway2.WANIPConnection2
	tasks.Go(func() error {
		var err error
		ip2Clients, _, err = internetgateway2.NewWANIPConnection2Clients()
		return err
	})
	var ppp1Clients []*internetgateway2.WANPPPConnection1
	tasks.Go(func() error {
		var err error
		ppp1Clients, _, err = internetgateway2.NewWANPPPConnection1Clients()
		return err
	})
	if err := tasks.Wait(); err != nil {
		return nil, err
	}
	// TODO provide more flexible handling if multiple devices are found
	switch {
	case len(ip2Clients) == 1:
		return ip2Clients[0], nil
	case len(ip1Clients) == 1:
		return ip1Clients[0], nil
	case len(ppp1Clients) == 1:
		return ppp1Clients[0], nil
	default:
		return nil, errors.New("multiple or no services found")
	}
}

func upnpGetExternalIP(ctx context.Context) string {
	client, err := pickRouterClient(ctx)
	if err != nil {
	}
	externalIP, err := client.GetExternalIPAddress()
	if err != nil {
	}
	return externalIP
}

func upnpForwardPort(ctx context.Context, port uint16) {
	client, err := pickRouterClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	externalIP := upnpGetExternalIP(ctx)
	client.AddPortMapping(
		"",
		port,
		"TCP",
		port,
		externalIP,
		true,
		"YourPlace",
		3600,
	)
}
