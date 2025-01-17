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

package messagebus

import (
	"fmt"

	"github.com/antoniomtz/app-functions-sdk-go/appcontext"
	"github.com/antoniomtz/app-functions-sdk-go/internal/common"
	"github.com/antoniomtz/app-functions-sdk-go/internal/runtime"
	"github.com/edgexfoundry/go-mod-core-contracts/clients"
	"github.com/edgexfoundry/go-mod-core-contracts/clients/coredata"
	"github.com/edgexfoundry/go-mod-core-contracts/clients/logger"
	"github.com/antoniomtz/go-mod-messaging/messaging"
	"github.com/antoniomtz/go-mod-messaging/pkg/types"
)

// Trigger implements Trigger to support MessageBusData
type Trigger struct {
	Configuration common.ConfigurationStruct
	Runtime       runtime.GolangRuntime
	logging       logger.LoggingClient
	client        messaging.MessageClient
	topics        []types.TopicChannel
	EventClient   coredata.EventClient
}

// Initialize ...
func (trigger *Trigger) Initialize(logger logger.LoggingClient) error {
	trigger.logging = logger
	logger.Info(fmt.Sprintf("Initializing Message Bus Trigger. Subscribing to topic: %s, Publish Topic: %s", trigger.Configuration.Binding.SubscribeTopic, trigger.Configuration.Binding.PublishTopic))
	var err error
	trigger.client, err = messaging.NewMessageClient(trigger.Configuration.MessageBus)

	if err != nil {
		return err
	}
	trigger.topics = []types.TopicChannel{{Topic: trigger.Configuration.Binding.SubscribeTopic, Messages: make(chan types.MessageEnvelope)}}
	messageErrors := make(chan error)

	trigger.client.Subscribe(trigger.topics, messageErrors)
	receiveMessage := true
	go func() {
		for receiveMessage {
			select {
			case msgErr := <-messageErrors:
				logger.Error(fmt.Sprintf("Failed to receive ZMQ Message, %v", msgErr))
			case msgs := <-trigger.topics[0].Messages:
				logger.Trace("Received message from bus", "topic", trigger.Configuration.Binding.PublishTopic, clients.CorrelationHeader, msgs.CorrelationID)

				edgexContext := &appcontext.Context{
					Configuration: trigger.Configuration,
					LoggingClient: trigger.logging,
					CorrelationID: msgs.CorrelationID,
					EventClient:   trigger.EventClient,
				}
				trigger.Runtime.ProcessEvent(edgexContext, msgs)
				if edgexContext.OutputData != nil {
					outputEnvelope := types.MessageEnvelope{
						CorrelationID: edgexContext.CorrelationID,
						Payload:       edgexContext.OutputData,
						ContentType:   clients.ContentTypeJSON,
					}
					err := trigger.client.Publish(outputEnvelope, trigger.Configuration.Binding.PublishTopic)
					if err != nil {
						logger.Error(fmt.Sprintf("Failed to publish Message to bus, %v", err))
					}

					logger.Trace("Published message to bus", "topic", trigger.Configuration.Binding.PublishTopic, clients.CorrelationHeader, msgs.CorrelationID)
				}
			}
		}
	}()

	return nil
}
