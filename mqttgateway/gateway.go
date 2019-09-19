package mqttgateway

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/underlx/disturbancesmlx/compute"
	"github.com/vmihailenco/msgpack"

	"github.com/gbl08ma/gmqtt"
	"github.com/gbl08ma/gmqtt/pkg/packets"
	"github.com/gbl08ma/keybox"
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// MQTTGateway is a real-time gateway that uses the MQTT protocol
type MQTTGateway struct {
	Log               *log.Logger
	Node              sqalx.Node
	vehicleHandler    *compute.VehicleHandler
	vehicleETAhandler *compute.VehicleETAHandler
	statsHandler      *compute.StatsHandler
	listenAddr        string
	wsListenAddr      string
	publicHost        string
	publicPort        int
	tlsCertPath       string
	tlsKeyPath        string
	authHashKey       []byte
	etaAvailability   string

	server   *gmqtt.Server
	stopChan chan interface{}
}

// MQTTGatewayStats contains stats about the
type MQTTGatewayStats struct {
	CurrentClients       int
	CurrentSubscriptions int
	TotalConnects        int
	TotalDisconnects     int
}

var stats MQTTGatewayStats

// Config contains runtime gateway configuration
type Config struct {
	Keybox            *keybox.Keybox
	Log               *log.Logger
	Node              sqalx.Node
	AuthHashKey       []byte
	VehicleHandler    *compute.VehicleHandler
	VehicleETAHandler *compute.VehicleETAHandler
	StatsHandler      *compute.StatsHandler
}

type userInfo struct {
	Pair        *dataobjects.APIPair
	IsWebSocket bool
	ConnectedAt time.Time
}

// New returns a new MQTTGateway with the specified settings
func New(c Config) (*MQTTGateway, error) {
	g := &MQTTGateway{
		Log:               c.Log,
		Node:              c.Node,
		vehicleHandler:    c.VehicleHandler,
		vehicleETAhandler: c.VehicleETAHandler,
		statsHandler:      c.StatsHandler,
		authHashKey:       c.AuthHashKey,
		stopChan:          make(chan interface{}, 1),
		etaAvailability:   "all",
	}
	var present, present2 bool
	g.listenAddr, present = c.Keybox.Get("listenAddr")
	if !present {
		return g, errors.New("Listening address not present in keybox")
	}

	g.wsListenAddr, present = c.Keybox.Get("wsListenAddr")
	if !present {
		return g, errors.New("WebCSocket listening address not present in keybox")
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

// Stats returns stats about the MQTT gateway
func (g *MQTTGateway) Stats() *MQTTGatewayStats {
	stats.CurrentClients = len(g.server.Monitor.Clients())
	stats.CurrentSubscriptions = len(g.server.Monitor.Subscriptions())
	return &MQTTGatewayStats{
		CurrentClients:       stats.CurrentClients,
		CurrentSubscriptions: stats.CurrentSubscriptions,
		TotalConnects:        stats.TotalConnects,
		TotalDisconnects:     stats.TotalDisconnects,
	}
}

// Start starts the MQTT gateway
func (g *MQTTGateway) Start() error {
	g.server = gmqtt.NewServer()
	g.stopChan = make(chan interface{}, 1)

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

	ws := &gmqtt.WsServer{
		Server: &http.Server{Addr: g.wsListenAddr},
	}
	g.server.AddWebSocketServer(ws)

	g.server.RegisterOnConnect(g.handleOnConnect)
	g.server.RegisterOnClose(g.handleOnClose)
	g.server.RegisterOnPublish(g.handleOnPublish)
	g.server.RegisterOnSubscribe(g.handleOnSubscribe)

	g.server.Run()

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		for {
			select {
			case <-ticker.C:
				err := g.SendVehicleETAs()
				if err != nil {
					g.Log.Println(err)
				}

				// disconnect clients that appear to be doing nothing
				for _, client := range g.server.Monitor.Clients() {
					if time.Since(client.ConnectedAt) > 30*time.Second && len(g.server.Monitor.ClientSubscriptions(client.ClientID)) == 0 {
						g.server.Client(client.ClientID).Close()
						g.Log.Println("Disconnected client", client.ClientID, "as it seemed idle")
					}
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

// HandleControlCommand handles a human-issued command to control the behavior of the gateway
// It returns a human-readable with the result
func (g *MQTTGateway) HandleControlCommand(command string, args ...string) string {
	switch command {
	case "setETAavailability":
		if len(args) != 1 {
			return "Invalid number of arguments"
		}
		if args[0] != "all" && args[0] != "none" && args[0] != "dev" {
			return "Argument must be one of `all`, `none` or `dev`"
		}
		g.etaAvailability = args[0]
		fallthrough
	case "getETAavailability":
		return "Vehicle ETA availability set to `" + g.etaAvailability + "`"
	default:
		return "Unknown MQTT control command `" + command + "`. Supported commands: `setETAavailability`, `getETAavailability`"
	}
}

func (g *MQTTGateway) handleOnConnect(client *gmqtt.Client) (code uint8) {
	key := client.ClientOptions().Username
	secret := client.ClientOptions().Password
	if key == "ws" {
		g.Log.Println("WebSocket client connected to the MQTT gateway")
		client.SetUserData(userInfo{
			IsWebSocket: true,
			ConnectedAt: time.Now(),
		})
		return packets.CodeAccepted
	}
	pair, err := dataobjects.GetPairIfCorrect(g.Node, key, secret, g.authHashKey)
	if err != nil {
		return packets.CodeBadUsernameorPsw
	}
	g.Log.Println("Pair", pair.Key, "connected to the MQTT gateway")
	client.SetUserData(userInfo{
		Pair:        pair,
		ConnectedAt: time.Now(),
	})
	stats.TotalConnects++
	return packets.CodeAccepted
}

func (g *MQTTGateway) handleOnClose(client *gmqtt.Client, err error) {
	stats.TotalDisconnects++
	if client.UserData() == nil {
		g.Log.Println("Unauthenticated client disconnected from the MQTT gateway")
		return
	}
	info := client.UserData().(userInfo)
	if info.IsWebSocket {
		g.Log.Println("WebSocket client disconnected from the MQTT gateway after being connected for", time.Now().Sub(info.ConnectedAt))
	} else {
		g.Log.Println("Pair", info.Pair.Key, "disconnected from the MQTT gateway after being connected for", time.Now().Sub(info.ConnectedAt))
	}
}

func (g *MQTTGateway) handleOnSubscribe(client *gmqtt.Client, topic packets.Topic) uint8 {
	if topic.Name == "test/nosubscribe" {
		return packets.SUBSCRIBE_FAILURE
	}
	if client.UserData() != nil {
		info := client.UserData().(userInfo)
		if info.IsWebSocket {
			g.Log.Println("WebSocket client subscribed to", topic.Name)
		} else {
			g.Log.Println("Pair", info.Pair.Key, "subscribed to", topic.Name)
		}
		g.Log.Println("Current subscriptions:")
		subs := g.server.Monitor.ClientSubscriptions(client.ClientOptions().ClientID)
		for _, sub := range subs {
			g.Log.Println("  " + sub.Name)
		}
		g.Log.Println("  " + topic.Name)

		switch g.etaAvailability {
		case "all":
			acceptableEncoding := "msgpack"
			if info.IsWebSocket {
				acceptableEncoding = "json"
			}
			if !strings.HasPrefix(topic.Name, acceptableEncoding+"/vehicleeta/") && !strings.HasPrefix(topic.Name, "dev-"+acceptableEncoding+"/vehicleeta/") {
				return packets.SUBSCRIBE_FAILURE
			}
		case "dev":
			if info.IsWebSocket {
				return packets.SUBSCRIBE_FAILURE
			}
			if !strings.HasPrefix(topic.Name, "dev-msgpack/vehicleeta/") {
				return packets.SUBSCRIBE_FAILURE
			}
		default:
			return packets.SUBSCRIBE_FAILURE
		}

		parts := strings.Split(topic.Name, "/")
		if len(parts) == 4 || (len(parts) == 5 && parts[4] == "all") {
			go func() {
				if !(client.UserData().(userInfo)).IsWebSocket {
					time.Sleep(1 * time.Second)
				}
				err := g.SendVehicleETAForStationToClient(client, topic.Name, parts[2], parts[3])
				if err != nil {
					g.Log.Println(err)
				}
			}()
		}
	}
	return topic.Qos
}

type payloadRealtimeLocation struct {
	StationID string `msgpack:"s" json:"s"`
	// DirectionID may be missing/empty if the user just entered the network
	DirectionID string `msgpack:"d" json:"d"`
}

func (g *MQTTGateway) handleOnPublish(client *gmqtt.Client, publish *packets.Publish) bool {
	if client.UserData() == nil {
		g.Log.Println("Unauthenticated client attempted publishing and will be disconnected")
		client.Close()
		return false
	}

	info := client.UserData().(userInfo)
	if info.IsWebSocket {
		g.Log.Println("WebSocket client attempted publishing and will be disconnected")
		client.Close()
		return false
	}
	// clients are only allowed to publish to some channels
	if strings.HasPrefix(string(publish.TopicName), "msgpack/rtloc/") || strings.HasPrefix(string(publish.TopicName), "dev-msgpack/rtloc/") {
		g.handleRealTimeLocationPublish(info, client, publish)
		return false
	}
	g.Log.Println("Pair", info.Pair.Key, "attempted publishing and will be disconnected")
	client.Close()
	return false
}

func (g *MQTTGateway) handleRealTimeLocationPublish(info userInfo, client *gmqtt.Client, publish *packets.Publish) {
	var request payloadRealtimeLocation
	err := msgpack.Unmarshal(publish.Payload, &request)
	if err != nil {
		g.Log.Println(err)
		return
	}

	tx, err := g.Node.Beginx()
	if err != nil {
		g.Log.Println(err)
		return
	}
	defer tx.Commit() // read-only tx

	station, err := dataobjects.GetStation(tx, request.StationID)
	if err != nil {
		g.Log.Println(err)
		return
	}

	lines, err := station.Lines(tx)
	if err != nil {
		g.Log.Println(err)
		return
	}

	if g.statsHandler != nil {
		if request.DirectionID == "" {
			g.statsHandler.RegisterActivity(lines, info.Pair, true)
		} else {
			g.statsHandler.RegisterActivity(lines, info.Pair, false)
		}
	}

	if g.vehicleHandler != nil {
		if request.DirectionID != "" {
			direction, err := dataobjects.GetStation(tx, request.DirectionID)
			if err != nil {
				g.Log.Println(err)
				return
			}

			g.vehicleHandler.RegisterTrainPassenger(station, direction)
			g.Log.Println("Received real-time location report through MQTT, station", station.ID, "direction", direction.ID)
		} else {
			g.Log.Println("Received real-time location report through MQTT, station", station.ID)
		}
	}
}
