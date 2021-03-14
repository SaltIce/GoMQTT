package test

import (
	"crypto/tls"
	"errors"
	MQTT "github.com/eclipse/paho.mqtt.golang"
	"log"
	"os"
	"os/signal"
	"testing"
	"time"
)

func TestM1(t *testing.T) {
	_ = NewAgent()
}

var MqttAgent *Agent

const (
	MQTT_QOS     = 3
	MQTTBroker   = "tcp://127.0.0.1:1883"
	MQTTClientId = "00000001"
)

// Agent runs an mqtt client
type Agent struct {
	client MQTT.Client
}

type subscription struct {
	topic   string
	handler MQTT.MessageHandler
}

// NewAgent creates an agent
func NewAgent() *Agent {

	a := new(Agent)
	//opts:= MQTT.NewClientOptions().AddBroker(MQTTconfig.MQTTBroker).SetClientID(MQTTconfig.MQTTClientId)
	opts := MQTT.NewClientOptions().AddBroker(MQTTBroker).SetClientID(MQTTClientId)
	//opts.SetUsername(MQTTconfig.MQTTUser)
	//opts.SetPassword(MQTTconfig.MQTTPasswd)
	opts.SetKeepAlive(5 * time.Second)
	opts.SetPingTimeout(5 * time.Second)
	opts.SetTLSConfig(&tls.Config{
		ClientAuth:         tls.NoClientCert,
		ClientCAs:          nil,
		InsecureSkipVerify: true,
	})

	opts.OnConnectionLost = func(c MQTT.Client, err error) {

	}

	a.client = MQTT.NewClient(opts)

	err := a.Connect()
	// mqtt.DEBUG = log.New(os.Stdout, "", 0)
	// mqtt.ERROR = log.New(os.Stdout, "", 0)
	log.Println(err)
	return a
}

// Connect opens a new connection
func (a *Agent) Connect() (err error) {
	token := a.client.Connect()
	if token.WaitTimeout(2*time.Second) == false {
		return errors.New("Open timeout")
	}
	if token.Error() != nil {
		return token.Error()
	}

	go func() {
		done := make(chan os.Signal)
		signal.Notify(done, os.Interrupt)
		<-done
		log.Println("Shutting down agent")
		a.Close()
	}()

	return
}

// Close agent
func (a *Agent) Close() {
	a.client.Disconnect(250)
}

// Publish things
func (a *Agent) Publish(topic string, retain bool, payload string) (err error) {
	token := a.client.Publish(topic, MQTT_QOS, retain, payload)
	if token.WaitTimeout(2*time.Second) == false {
		return errors.New("Publish timout")
	}
	if token.Error() != nil {
		return token.Error()
	}
	return nil
}
