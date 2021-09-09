/*
Copyright 2015 Gravitational, Inc.

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

package service

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/srv/app"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := MakeDefaultConfig()
	require.NotNil(t, config)

	// all 3 services should be enabled by default
	require.True(t, config.Auth.Enabled)
	require.True(t, config.SSH.Enabled)
	require.True(t, config.Proxy.Enabled)

	localAuthAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:3025"}
	localProxyAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:3023"}

	// data dir, hostname and auth server
	require.Equal(t, config.DataDir, defaults.DataDir)
	if len(config.Hostname) < 2 {
		t.Fatal("default hostname wasn't properly set")
	}

	// crypto settings
	require.Equal(t, config.CipherSuites, utils.DefaultCipherSuites())
	// Unfortunately the below algos don't have exported constants in
	// golang.org/x/crypto/ssh for us to use.
	require.Equal(t, config.Ciphers, []string{
		"aes128-gcm@openssh.com",
		"chacha20-poly1305@openssh.com",
		"aes128-ctr",
		"aes192-ctr",
		"aes256-ctr",
	})
	require.Equal(t, config.KEXAlgorithms, []string{
		"curve25519-sha256@libssh.org",
		"ecdh-sha2-nistp256",
		"ecdh-sha2-nistp384",
		"ecdh-sha2-nistp521",
	})
	require.Equal(t, config.MACAlgorithms, []string{
		"hmac-sha2-256-etm@openssh.com",
		"hmac-sha2-256",
	})
	require.Nil(t, config.CASignatureAlgorithm)

	// auth section
	auth := config.Auth
	require.Equal(t, auth.SSHAddr, localAuthAddr)
	require.Equal(t, auth.Limiter.MaxConnections, int64(defaults.LimiterMaxConnections))
	require.Equal(t, auth.Limiter.MaxNumberOfUsers, defaults.LimiterMaxConcurrentUsers)
	require.Equal(t, config.Auth.StorageConfig.Type, lite.GetName())
	require.Equal(t, auth.StorageConfig.Params[defaults.BackendPath], filepath.Join(config.DataDir, defaults.BackendDir))

	// SSH section
	ssh := config.SSH
	require.Equal(t, ssh.Limiter.MaxConnections, int64(defaults.LimiterMaxConnections))
	require.Equal(t, ssh.Limiter.MaxNumberOfUsers, defaults.LimiterMaxConcurrentUsers)
	require.Equal(t, ssh.AllowTCPForwarding, true)

	// proxy section
	proxy := config.Proxy
	require.Equal(t, proxy.SSHAddr, localProxyAddr)
	require.Equal(t, proxy.Limiter.MaxConnections, int64(defaults.LimiterMaxConnections))
	require.Equal(t, proxy.Limiter.MaxNumberOfUsers, defaults.LimiterMaxConcurrentUsers)
}

// TestCheckApp validates application configuration.
func TestCheckApp(t *testing.T) {
	type tc struct {
		desc  string
		inApp App
		err   string
	}
	tests := []tc{
		{
			desc: "valid subdomain",
			inApp: App{
				Name: "foo",
				URI:  "http://localhost",
			},
		},
		{
			desc: "subdomain cannot start with a dash",
			inApp: App{
				Name: "-foo",
				URI:  "http://localhost",
			},
			err: "must be a valid DNS subdomain",
		},
		{
			desc: `subdomain cannot contain the exclamation mark character "!"`,
			inApp: App{
				Name: "foo!bar",
				URI:  "http://localhost",
			},
			err: "must be a valid DNS subdomain",
		},
		{
			desc: "subdomain of length 63 characters is valid (maximum length)",
			inApp: App{
				Name: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				URI:  "http://localhost",
			},
		},
		{
			desc: "subdomain of length 64 characters is invalid",
			inApp: App{
				Name: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				URI:  "http://localhost",
			},
			err: "must be a valid DNS subdomain",
		},
	}
	for _, h := range app.ReservedHeaders {
		tests = append(tests, tc{
			desc: fmt.Sprintf("reserved header rewrite %v", h),
			inApp: App{
				Name: "foo",
				URI:  "http://localhost",
				Rewrite: &Rewrite{
					Headers: []Header{
						{
							Name:  h,
							Value: "rewritten",
						},
					},
				},
			},
			err: `invalid application "foo" header rewrite configuration`,
		})
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := tt.inApp.Check()
			if tt.err != "" {
				require.Contains(t, err.Error(), tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckDatabase(t *testing.T) {
	tests := []struct {
		desc       string
		inDatabase Database
		outErr     bool
	}{
		{
			desc: "ok",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			},
			outErr: false,
		},
		{
			desc: "empty database name",
			inDatabase: Database{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			},
			outErr: true,
		},
		{
			desc: "invalid database name",
			inDatabase: Database{
				Name:     "??--++",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			},
			outErr: true,
		},
		{
			desc: "invalid database protocol",
			inDatabase: Database{
				Name:     "example",
				Protocol: "unknown",
				URI:      "localhost:5432",
			},
			outErr: true,
		},
		{
			desc: "invalid database uri",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost",
			},
			outErr: true,
		},
		{
			desc: "invalid database CA cert",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				CACert:   []byte("cert"),
			},
			outErr: true,
		},
		{
			desc: "GCP valid configuration",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				GCP: DatabaseGCP{
					ProjectID:  "project-1",
					InstanceID: "instance-1",
				},
				CACert: fixtures.LocalhostCert,
			},
			outErr: false,
		},
		{
			desc: "GCP project ID specified without instance ID",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				GCP: DatabaseGCP{
					ProjectID: "project-1",
				},
				CACert: fixtures.LocalhostCert,
			},
			outErr: true,
		},
		{
			desc: "GCP instance ID specified without project ID",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				GCP: DatabaseGCP{
					InstanceID: "instance-1",
				},
				CACert: fixtures.LocalhostCert,
			},
			outErr: true,
		},
		{
			desc: "MongoDB connection string",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolMongoDB,
				URI:      "mongodb://mongo-1:27017,mongo-2:27018/?replicaSet=rs0",
			},
			outErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.inDatabase.CheckAndSetDefaults()
			if test.outErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestParseHeaders validates parsing of strings into http header objects.
func TestParseHeaders(t *testing.T) {
	tests := []struct {
		desc string
		in   []string
		out  []Header
		err  string
	}{
		{
			desc: "parse multiple headers",
			in: []string{
				"Host: example.com    ",
				"X-Teleport-Logins: root, {{internal.logins}}",
				"X-Env  : {{external.env}}",
				"X-Env: env:prod",
				"X-Empty:",
			},
			out: []Header{
				{Name: "Host", Value: "example.com"},
				{Name: "X-Teleport-Logins", Value: "root, {{internal.logins}}"},
				{Name: "X-Env", Value: "{{external.env}}"},
				{Name: "X-Env", Value: "env:prod"},
				{Name: "X-Empty", Value: ""},
			},
		},
		{
			desc: "invalid header format (missing value)",
			in:   []string{"X-Header"},
			err:  `failed to parse "X-Header" as http header`,
		},
		{
			desc: "invalid header name (empty)",
			in:   []string{": missing"},
			err:  `invalid http header name: ": missing"`,
		},
		{
			desc: "invalid header name (space)",
			in:   []string{"X Space: space"},
			err:  `invalid http header name: "X Space: space"`,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			out, err := ParseHeaders(test.in)
			if test.err != "" {
				require.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.out, out)
			}
		})
	}
}
