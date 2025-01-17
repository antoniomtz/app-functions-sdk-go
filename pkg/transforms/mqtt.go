//
// Copyright (c) 2019 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package transforms

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strconv"
	"strings"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/antoniomtz/app-functions-sdk-go/appcontext"
	"github.com/edgexfoundry/go-mod-core-contracts/clients"
	"github.com/edgexfoundry/go-mod-core-contracts/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/models"
)

// MqttConfig contains mqtt client parameters
type MqttConfig struct {
	qos           byte
	retain        bool
	autoreconnect bool
}

type MQTTSender struct {
	client MQTT.Client
	topic  string
	opts   MqttConfig
}

// NewMqttConfig returns a new MqttConfig with default values
func NewMqttConfig() *MqttConfig {
	mqttConfig := new(MqttConfig)
	mqttConfig.qos = 0
	mqttConfig.retain = false
	mqttConfig.autoreconnect = false

	return mqttConfig
}

// SetRetain enables or disables mqtt retain option
func (mqttConfig MqttConfig) SetRetain(retain bool) {
	mqttConfig.retain = retain
}

// SetQos changes mqtt qos(0,1,2) for all messages
func (mqttConfig MqttConfig) SetQos(qos byte) {
	mqttConfig.qos = qos
}

// SetAutoreconnect enables or disables the automatic client reconnection to broker
func (mqttConfig MqttConfig) SetAutoreconnect(reconnect bool) {
	mqttConfig.autoreconnect = reconnect
}

// MQTTSend ...
func (sender MQTTSender) MQTTSend(edgexcontext *appcontext.Context, params ...interface{}) (bool, interface{}) {
	if len(params) < 1 {
		// We didn't receive a result
		return false, errors.New("No Data Received")
	}
	if !sender.client.IsConnected() {
		edgexcontext.LoggingClient.Info("Connecting to mqtt server")
		if token := sender.client.Connect(); token.Wait() && token.Error() != nil {
			return false, fmt.Errorf("Could not connect to mqtt server, drop event. Error: %s", token.Error().Error())
		}
		edgexcontext.LoggingClient.Info("Connected to mqtt server")
	}
	if data, ok := params[0].(string); ok {
		token := sender.client.Publish(sender.topic, sender.opts.qos, sender.opts.retain, ([]byte)(data))
		// FIXME: could be removed? set of tokens?
		token.Wait()
		if token.Error() != nil {
			return false, token.Error()
		}
		edgexcontext.LoggingClient.Info("Sent data to MQTT Broker")
		edgexcontext.LoggingClient.Trace("Data exported", "Transport", "MQTT", clients.CorrelationHeader, edgexcontext.CorrelationID)
		err := edgexcontext.MarkAsPushed()
		if err != nil {
			edgexcontext.LoggingClient.Error(err.Error())
		}
		return true, nil

	}
	return false, errors.New("Unexpected type received")
}

// NewMQTTSender - create new mqtt sender
func NewMQTTSender(logging logger.LoggingClient, addr models.Addressable, certFile string, key string, config *MqttConfig) *MQTTSender {
	protocol := strings.ToLower(addr.Protocol)

	opts := MQTT.NewClientOptions()
	broker := protocol + "://" + addr.Address + ":" + strconv.Itoa(addr.Port) + addr.Path
	opts.AddBroker(broker)
	opts.SetClientID(addr.Publisher)
	opts.SetUsername(addr.User)
	opts.SetPassword(addr.Password)
	opts.SetAutoReconnect(config.autoreconnect)

	if protocol == "tcps" || protocol == "ssl" || protocol == "tls" {
		cert, err := tls.LoadX509KeyPair(certFile, key)

		if err != nil {
			logging.Error("Failed loading x509 data")
			return nil
		}

		tlsConfig := &tls.Config{
			ClientCAs:          nil,
			InsecureSkipVerify: true,
			Certificates:       []tls.Certificate{cert},
		}

		opts.SetTLSConfig(tlsConfig)

	}

	sender := &MQTTSender{
		client: MQTT.NewClient(opts),
		topic:  addr.Topic,
		opts:   *config,
	}

	return sender
}
