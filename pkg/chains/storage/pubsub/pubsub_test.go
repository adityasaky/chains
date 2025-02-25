/*
Copyright 2022 The Tekton Authors
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"

	"github.com/in-toto/in-toto-golang/in_toto"
	"github.com/tektoncd/chains/pkg/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"gocloud.dev/pubsub"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logtesting "knative.dev/pkg/logging/testing"
)

func TestBackend_StorePayload(t *testing.T) {
	// pretty much anything that has no Subject
	sampleIntotoStatementBytes, _ := json.Marshal(in_toto.Statement{})
	logger := logtesting.TestLogger(t)

	type fields struct {
		tr  *v1beta1.TaskRun
		cfg config.Config
	}
	type args struct {
		rawPayload  []byte
		signature   string
		storageOpts config.StorageOpts
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "no subject",
			fields: fields{
				tr: &v1beta1.TaskRun{
					ObjectMeta: v1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
					},
				},
				cfg: config.Config{
					Storage: config.StorageConfigs{
						PubSub: config.PubSubStorageConfig{
							Provider: "inmemory",
							Topic:    "test",
						},
					},
				},
			},
			args: args{
				rawPayload: sampleIntotoStatementBytes,
				signature:  "signature",
				storageOpts: config.StorageOpts{
					PayloadFormat: "in-toto",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Backend{
				logger: logger,
				cfg:    tt.fields.cfg,
			}
			addr := fmt.Sprintf("mem://%s", tt.fields.cfg.Storage.PubSub.Topic)
			ctx := context.Background()

			// Create the test topic.
			topic, err := pubsub.OpenTopic(ctx, addr)
			if err != nil {
				t.Errorf("could not open topic: %v", err)
			}
			defer func() {
				if err := topic.Shutdown(ctx); err != nil {
					logger.Error(err)
				}
			}()

			// Subscribe to the pubsub.
			sub, err := pubsub.OpenSubscription(ctx, addr)
			if err != nil {
				log.Fatal(err)
			}
			defer func() {
				if err := sub.Shutdown(ctx); err != nil {
					logger.Error(err)
				}
			}()

			// Store the payload.
			if err := b.StorePayload(ctx, tt.fields.tr, tt.args.rawPayload, tt.args.signature, tt.args.storageOpts); (err != nil) != tt.wantErr {
				t.Errorf("Backend.StorePayload() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Retrieve the payload from the pubsub.
			msg, err := sub.Receive(ctx)
			if err != nil {
				log.Fatal(err)
			}

			// Compare the results.
			got := string(msg.Body)
			want := tt.args.signature
			if got != want {
				t.Errorf("error retrieving the message body, want: %v, got: %v", want, got)
			}
			msg.Ack()
		})
	}
}
