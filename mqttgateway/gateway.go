package mqttgateway

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/underlx/disturbancesmlx/compute"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"github.com/gbl08ma/keybox"
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// MQTTGateway is a real-time gateway that uses the MQTT protocol
type MQTTGateway struct {
	Log            *log.Logger
	Node           sqalx.Node
	vehicleHandler *compute.VehicleHandler
	listenAddr     string
	publicHost     string
	publicPort     int
	tlsCertPath    string
	tlsKeyPath     string
	authHashKey    []byte

	server   *gmqtt.Server
	stopChan chan interface{}
}

// Config contains runtime gateway configuration
type Config struct {
	Keybox         *keybox.Keybox
	Log            *log.Logger
	Node           sqalx.Node
	AuthHashKey    []byte
	VehicleHandler *compute.VehicleHandler
}

type userInfo struct {
	Pair        *dataobjects.APIPair
	ConnectTime time.Time
}

// New returns a new MQTTGateway with the specified settings
func New(c Config) (*MQTTGateway, error) {
	g := &MQTTGateway{
		Log:            c.Log,
		Node:           c.Node,
		vehicleHandler: c.VehicleHandler,
		authHashKey:    c.AuthHashKey,
		stopChan:       make(chan interface{}, 1),
	}
	var present, present2 bool
	g.listenAddr, present = c.Keybox.Get("listenAddr")
	if !present {
		return g, errors.New("Listening address not present in keybox")
	}

	g.publicHost, present = c.Keybox.Get("hostname")
	if !present {
		return g, errors.New("Hostname not present in keybox")
	}
	port, present := c.Keybox.Get("port")
	if !present {
		return g, errors.New("Port not present in keybox")
	}
	var err error
	g.publicPort, err = strconv.Atoi(port)
	if err != nil {
		return g, err
	}

	g.tlsCertPath, present = c.Keybox.Get("certPath")
	g.tlsKeyPath, present2 = c.Keybox.Get("keyPath")
	if !present || !present2 {
		g.Log.Println("TLS cert/key paths not present in keybox, will not use TLS")
	}

	return g, nil
}

// Start starts the MQTT gateway
func (g *MQTTGateway) Start() error {
	g.server = gmqtt.NewServer()

	var ln net.Listener
	var err error
	if g.IsTLS() {
		crt, err := tls.LoadX509KeyPair(g.tlsCertPath, g.tlsKeyPath)
		if err != nil {
			return err
		}
		tlsConfig := &tls.Config{}
		tlsConfig.Certificates = []tls.Certificate{crt}
		ln, err = tls.Listen("tcp", g.listenAddr, tlsConfig)
	} else {
		ln, err = net.Listen("tcp", g.listenAddr)
	}
	if err != nil {
		return err
	}
	g.server.AddTCPListenner(ln)

	g.server.RegisterOnConnect(g.handleOnConnect)
	g.server.RegisterOnClose(g.handleOnClose)
	g.server.RegisterOnPublish(g.handleOnPublish)

	g.server.RegisterOnSubscribe(func(client *gmqtt.Client, topic packets.Topic) uint8 {
		if topic.Name == "test/nosubscribe" {
			return packets.SUBSCRIBE_FAILURE
		}
		if client.UserData() != nil {
			info := client.UserData().(userInfo)
			g.Log.Println("Pair", info.Pair.Key, "subscribed to", topic.Name)
			g.Log.Println("Current subscriptions:")
			subs := g.server.Monitor.ClientSubscriptions(client.ClientOptions().ClientID)
			for _, sub := range subs {
				g.Log.Println("  " + sub.Name)
			}
		}
		return topic.Qos
	})

	g.server.Run()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		for {
			select {
			case <-ticker.C:
				err := g.SendVehicleETAs()
				if err != nil {
					g.Log.Println(err)
				}
			case <-g.stopChan:
				return
			}
		}
	}()
	g.Log.Println("MQTT broker started")
	return nil
}

// Hostname returns the hostname where the gateway can be contacted
func (g *MQTTGateway) Hostname() string {
	return g.publicHost
}

// Port returns the port where the gateway can be contacted
func (g *MQTTGateway) Port() uint16 {
	return uint16(g.publicPort)
}

// IsTLS returns whether this gateway operates over TLS
func (g *MQTTGateway) IsTLS() bool {
	return g.tlsCertPath != "" && g.tlsKeyPath != ""
}

// MQTTVersion returns the MQTT version supported by this gateway
func (g *MQTTGateway) MQTTVersion() string {
	return "3.1.1"
}

// Stop stops the MQTT gateway
func (g *MQTTGateway) Stop() error {
	g.stopChan <- true
	return g.server.Stop(context.Background())
}

func (g *MQTTGateway) handleOnConnect(client *gmqtt.Client) (code uint8) {
	key := client.ClientOptions().Username
	secret := client.ClientOptions().Password
	pair, err := dataobjects.GetPairIfCorrect(g.Node, key, secret, g.authHashKey)
	if err != nil {
		return packets.CodeBadUsernameorPsw
	}
	g.Log.Println("Pair", pair.Key, "connected to the MQTT gateway")
	client.SetUserData(userInfo{
		Pair:        pair,
		ConnectTime: time.Now(),
	})
	return packets.CodeAccepted
}

func (g *MQTTGateway) handleOnClose(client *gmqtt.Client, err error) {
	if client.UserData() == nil {
		g.Log.Println("Unauthenticated client disconnected from the MQTT gateway")
		return
	}
	info := client.UserData().(userInfo)
	g.Log.Println("Pair", info.Pair.Key, "disconnected from the MQTT gateway after being connected for", time.Now().Sub(info.ConnectTime))
}

func (g *MQTTGateway) handleOnPublish(client *gmqtt.Client, publish *packets.Publish) bool {
	if client.UserData() == nil {
		g.Log.Println("Unauthenticated client attempted publishing and will be disconnected")
		client.Close()
		return false
	}

	info := client.UserData().(userInfo)
	// for now, clients are not allowed to publish to any channel
	g.Log.Println("Pair", info.Pair.Key, "attempted publishing and will be disconnected")
	client.Close()
	return false
}
