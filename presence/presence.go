package presence

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/c0deaddict/arpwatch/utils"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

var (
	host         = flag.String("mqtt-host", "127.0.0.1", "MQTT host")
	port         = utils.PortFlag("mqtt-port", 1883, "MQTT port")
	username     = flag.String("mqtt-username", "", "MQTT username")
	password     = flag.String("mqtt-password", "", "MQTT password")
	passwordFile = flag.String("mqtt-password-file", "", "MQTT password file (takes precedence over mqtt-password)")
	clientID     = flag.String("mqtt-client-id", "arpwatch", "MQTT client ID")

	client mqtt.Client
)

func Connect() error {
	opts := mqtt.NewClientOptions()
	server := fmt.Sprintf("tcp://%s:%d", *host, *port)
	opts.AddBroker(server)
	opts.SetClientID(*clientID)

	opts.SetUsername(*username)
	if password, err := readPassword(); err != nil {
		return err
	} else {
		opts.SetPassword(*password)
	}

	opts.OnConnect = func(client mqtt.Client) {
		log.Println("Connected to MQTT")
	}
	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Printf("Connection to MQTT lost: %v", err)
	}

	log.Printf("Connecting to MQTT server at %v", server)
	client = mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func Close() {
	client.Disconnect(250)
}

func HostOnline(mac net.HardwareAddr, ip net.IP) {
}

func HostOffline(mac net.HardwareAddr, ip net.IP) {
}

func NewHost(mac net.HardwareAddr, ip net.IP) {
}

func readPassword() (*string, error) {
	if *passwordFile != "" {
		if password, err := utils.ReadFirstLine(*passwordFile); err != nil {
			return nil, err
		} else {
			return password, nil
		}
	}

	return password, nil
}
