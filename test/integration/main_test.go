//go:build integration

/*
 * Copyright 2020 The Yorkie Authors. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/yorkie-team/yorkie/api/types"
	"github.com/yorkie-team/yorkie/client"
	"github.com/yorkie-team/yorkie/pkg/document"
	"github.com/yorkie-team/yorkie/server"
	"github.com/yorkie-team/yorkie/server/logging"
	"github.com/yorkie-team/yorkie/test/helper"
)

type clientAndDocPair struct {
	cli *client.Client
	doc *document.Document
}

type watchResponsePair struct {
	Type  client.WatchResponseType
	Peers map[string]types.Metadata
}

var defaultServer *server.Yorkie

func TestMain(m *testing.M) {
	svr := helper.TestServer()
	if err := svr.Start(); err != nil {
		logging.DefaultLogger().Fatal(err)
	}
	defaultServer = svr
	code := m.Run()
	if defaultServer != nil {
		if err := defaultServer.Shutdown(true); err != nil {
			logging.DefaultLogger().Error(err)
		}
	}
	os.Exit(code)
}

func syncClientsThenAssertEqual(t *testing.T, pairs []clientAndDocPair) {
	assert.True(t, len(pairs) > 1)
	ctx := context.Background()
	// Save own changes and get previous changes.
	for i, pair := range pairs {
		fmt.Printf("before d%d: %s\n", i+1, pair.doc.Marshal())
		err := pair.cli.Sync(ctx)
		assert.NoError(t, err)
	}

	// Get last client changes.
	// Last client get all precede changes in above loop.
	for _, pair := range pairs[:len(pairs)-1] {
		err := pair.cli.Sync(ctx)
		assert.NoError(t, err)
	}

	// Assert start.
	expected := pairs[0].doc.Marshal()
	fmt.Printf("after d1: %s\n", expected)
	for i, pair := range pairs[1:] {
		v := pair.doc.Marshal()
		fmt.Printf("after d%d: %s\n", i+2, v)
		assert.Equal(t, expected, v)
	}
}

// clientConn is a helper function to create a client connection.
func clientConn() (*grpc.ClientConn, error) {
	conn, err := grpc.Dial(
		defaultServer.RPCAddr(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

// activeClient is a helper function to create active clients.
func activeClients(t *testing.T, n int) (clients []*client.Client) {
	for i := 0; i < n; i++ {
		c, err := client.Dial(
			defaultServer.RPCAddr(),
			client.WithMetadata(types.Metadata{"name": fmt.Sprintf("name-%d", i)}),
		)
		assert.NoError(t, err)

		err = c.Activate(context.Background())
		assert.NoError(t, err)

		clients = append(clients, c)
	}
	return
}

// cleanupClients is a helper function to clean up clients.
func cleanupClients(t *testing.T, clients []*client.Client) {
	for _, c := range clients {
		assert.NoError(t, c.Deactivate(context.Background()))
		assert.NoError(t, c.Close())
	}
}
