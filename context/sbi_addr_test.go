// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package context

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/omec-project/ausf/factory"
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/util/sbiaddr"
)

const (
	testPort    = 29509
	testIPv4    = "10.0.0.5"
	testIPv6    = "fd00::5"
	testFQDN    = "ausf"
	testV4URI   = "https://10.0.0.5:29509"
	testV6URI   = "https://[fd00::5]:29509"
	testDefault = "https://" + factory.AUSF_DEFAULT_IPV4 + ":29509"
	testPodIPv6 = "2001:db8:600::5"
	testAnyV4   = "0.0.0.0"
)

// endPoint is the part of an IpEndPoint the tests compare.
type endPoint struct {
	ipv4, ipv6 string
	port       int32
}

func endPoints(svc models.NFService) []endPoint {
	var out []endPoint
	for _, ep := range svc.GetIpEndPoints() {
		out = append(out, endPoint{ipv4: ep.GetIpv4Address(), ipv6: ep.GetIpv6Address(), port: ep.GetPort()})
	}
	return out
}

func initWithSbi(t *testing.T, sbi *factory.Sbi) *AUSFContext {
	t.Helper()
	orig := factory.AusfConfig
	t.Cleanup(func() { factory.AusfConfig = orig })
	factory.AusfConfig = factory.Config{
		Info:          &factory.Info{Version: "1.0.0"},
		Configuration: &factory.Configuration{Sbi: sbi},
	}
	var ctx AUSFContext
	InitAusfContext(&ctx)
	return &ctx
}

func TestInitAusfContextRegisterAddrs(t *testing.T) {
	tests := []struct {
		name           string
		env            map[string]string
		sbi            factory.Sbi
		wantIPv4       string // context.RegisterIPv4
		wantIPv6       string // context.RegisterIPv6
		wantURL        string
		wantFqdn       string // NFService.fqdn
		wantEndPoints  []endPoint
		wantPreference sbiaddr.Preference
	}{
		{
			name:          "IPv4",
			sbi:           factory.Sbi{RegisterIPv4: testIPv4},
			wantIPv4:      testIPv4,
			wantURL:       testV4URI,
			wantEndPoints: []endPoint{{ipv4: testIPv4, port: testPort}},
		},
		{
			name:          "FQDN goes to fqdn, endpoint keeps only the port",
			sbi:           factory.Sbi{RegisterIPv4: testFQDN},
			wantIPv4:      testFQDN,
			wantURL:       "https://ausf:29509",
			wantFqdn:      testFQDN,
			wantEndPoints: []endPoint{{port: testPort}},
		},
		{
			name:          "IPv6 under registerIPv4 goes to the IPv6 slot",
			sbi:           factory.Sbi{RegisterIPv4: testIPv6},
			wantIPv6:      testIPv6,
			wantURL:       testV6URI,
			wantEndPoints: []endPoint{{ipv6: testIPv6, port: testPort}},
		},
		{
			name:          "registerIPv6 only",
			sbi:           factory.Sbi{RegisterIPv6: "[FD00:0::5]"},
			wantIPv6:      testIPv6,
			wantURL:       testV6URI,
			wantEndPoints: []endPoint{{ipv6: testIPv6, port: testPort}},
		},
		{
			name:          "dual stack: two endpoints, IPv4 first, IPv4 URI by default",
			sbi:           factory.Sbi{RegisterIPv4: testIPv4, RegisterIPv6: testIPv6},
			wantIPv4:      testIPv4,
			wantIPv6:      testIPv6,
			wantURL:       testV4URI,
			wantEndPoints: []endPoint{{ipv4: testIPv4, port: testPort}, {ipv6: testIPv6, port: testPort}},
		},
		{
			name:           "dual stack with preferredFamily IPv6",
			sbi:            factory.Sbi{RegisterIPv4: testIPv4, RegisterIPv6: testIPv6, PreferredFamily: "IPv6"},
			wantIPv4:       testIPv4,
			wantIPv6:       testIPv6,
			wantURL:        testV6URI,
			wantEndPoints:  []endPoint{{ipv4: testIPv4, port: testPort}, {ipv6: testIPv6, port: testPort}},
			wantPreference: sbiaddr.PreferIPv6,
		},
		{
			name:          "unknown preferredFamily falls back to IPv4",
			sbi:           factory.Sbi{RegisterIPv4: testIPv4, RegisterIPv6: testIPv6, PreferredFamily: "IPv5"},
			wantIPv4:      testIPv4,
			wantIPv6:      testIPv6,
			wantURL:       testV4URI,
			wantEndPoints: []endPoint{{ipv4: testIPv4, port: testPort}, {ipv6: testIPv6, port: testPort}},
		},
		{
			name:          "FQDN plus IPv6",
			sbi:           factory.Sbi{RegisterIPv4: testFQDN, RegisterIPv6: testIPv6},
			wantIPv4:      testFQDN,
			wantIPv6:      testIPv6,
			wantURL:       "https://ausf:29509",
			wantFqdn:      testFQDN,
			wantEndPoints: []endPoint{{ipv6: testIPv6, port: testPort}},
		},
		{
			name:          "POD_IP holding IPv6 under registerIPv4",
			env:           map[string]string{sbiaddr.EnvPodIP: testPodIPv6},
			sbi:           factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIP},
			wantIPv6:      testPodIPv6,
			wantURL:       "https://[" + testPodIPv6 + "]:29509",
			wantEndPoints: []endPoint{{ipv6: testPodIPv6, port: testPort}},
		},
		{
			name:          "POD_IP holding IPv4 under registerIPv4",
			env:           map[string]string{sbiaddr.EnvPodIP: testIPv4},
			sbi:           factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIP},
			wantIPv4:      testIPv4,
			wantURL:       testV4URI,
			wantEndPoints: []endPoint{{ipv4: testIPv4, port: testPort}},
		},
		{
			name:          "POD_IPS dual stack",
			env:           map[string]string{sbiaddr.EnvPodIPs: "fd00::5,10.0.0.5"},
			sbi:           factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIPs},
			wantIPv4:      testIPv4,
			wantIPv6:      testIPv6,
			wantURL:       testV4URI,
			wantEndPoints: []endPoint{{ipv4: testIPv4, port: testPort}, {ipv6: testIPv6, port: testPort}},
		},
		{
			name:          "nothing configured: the AUSF default",
			sbi:           factory.Sbi{},
			wantIPv4:      factory.AUSF_DEFAULT_IPV4,
			wantURL:       testDefault,
			wantEndPoints: []endPoint{{ipv4: factory.AUSF_DEFAULT_IPV4, port: testPort}},
		},
		{
			name:          "unset POD_IP: the AUSF default",
			sbi:           factory.Sbi{RegisterIPv4: sbiaddr.EnvPodIP},
			wantIPv4:      factory.AUSF_DEFAULT_IPV4,
			wantURL:       testDefault,
			wantEndPoints: []endPoint{{ipv4: factory.AUSF_DEFAULT_IPV4, port: testPort}},
		},
		{
			name:          "invalid value: the AUSF default",
			sbi:           factory.Sbi{RegisterIPv4: "10.0.0.256"},
			wantIPv4:      factory.AUSF_DEFAULT_IPV4,
			wantURL:       testDefault,
			wantEndPoints: []endPoint{{ipv4: factory.AUSF_DEFAULT_IPV4, port: testPort}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(sbiaddr.EnvPodIP, "")
			t.Setenv(sbiaddr.EnvPodIPs, "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			sbi := tt.sbi
			sbi.Scheme = "https"
			sbi.Port = testPort
			ctx := initWithSbi(t, &sbi)

			if ctx.RegisterIPv4 != tt.wantIPv4 || ctx.RegisterIPv6 != tt.wantIPv6 {
				t.Errorf("RegisterIPv4, RegisterIPv6 = %q, %q, want %q, %q",
					ctx.RegisterIPv4, ctx.RegisterIPv6, tt.wantIPv4, tt.wantIPv6)
			}
			if ctx.SbiPreference != tt.wantPreference {
				t.Errorf("SbiPreference = %v, want %v", ctx.SbiPreference, tt.wantPreference)
			}
			if ctx.Url != tt.wantURL {
				t.Errorf("Url = %q, want %q", ctx.Url, tt.wantURL)
			}
			if got := ctx.GetSbiUri(); got != tt.wantURL {
				t.Errorf("GetSbiUri() = %q, want %q", got, tt.wantURL)
			}
			if got := ctx.GetIPv4Uri(); got != tt.wantURL {
				t.Errorf("GetIPv4Uri() = %q, want %q", got, tt.wantURL)
			}
			if u, err := url.Parse(ctx.Url); err != nil || u.Port() != "29509" {
				t.Errorf("url.Parse(%q) = %v, %v; want port 29509", ctx.Url, u, err)
			}

			svc, ok := ctx.NfService[models.SERVICENAME_NAUSF_AUTH]
			if !ok {
				t.Fatalf("no %s service", models.SERVICENAME_NAUSF_AUTH)
			}
			if got := svc.GetApiPrefix(); got != tt.wantURL {
				t.Errorf("service apiPrefix = %q, want %q", got, tt.wantURL)
			}
			if got := svc.GetFqdn(); got != tt.wantFqdn {
				t.Errorf("service fqdn = %q, want %q", got, tt.wantFqdn)
			}
			got := endPoints(svc)
			if len(got) != len(tt.wantEndPoints) {
				t.Fatalf("endpoints = %+v, want %+v", got, tt.wantEndPoints)
			}
			for i := range got {
				if got[i] != tt.wantEndPoints[i] {
					t.Errorf("endpoint %d = %+v, want %+v", i, got[i], tt.wantEndPoints[i])
				}
			}
		})
	}
}

func TestInitAusfContextBindingAddr(t *testing.T) {
	const bindEnv = "AUSF_TEST_BIND"
	tests := []struct {
		name string
		env  string
		sbi  factory.Sbi
		want string
	}{
		{name: "default", want: sbiaddr.DefaultBindHost},
		{name: "bindingIPv4 IPv4", sbi: factory.Sbi{BindingIPv4: testAnyV4}, want: testAnyV4},
		{name: "bindingIPv4 IPv6 wildcard", sbi: factory.Sbi{BindingIPv4: "::"}, want: "::"},
		{name: "bindingAddr", sbi: factory.Sbi{BindingAddr: "::"}, want: "::"},
		{name: "bindingAddr wins over bindingIPv4", sbi: factory.Sbi{BindingAddr: "fd00::5", BindingIPv4: testAnyV4}, want: testIPv6},
		{name: "bracketed bindingAddr", sbi: factory.Sbi{BindingAddr: "[fd00::5]"}, want: testIPv6},
		{name: "bindingIPv4 from env", env: "fd00::5", sbi: factory.Sbi{BindingIPv4: bindEnv}, want: testIPv6},
		{name: "bindingAddr from env", env: testIPv4, sbi: factory.Sbi{BindingAddr: bindEnv}, want: testIPv4},
		{name: "a list falls back to the wildcard", env: "10.0.0.5,fd00::5", sbi: factory.Sbi{BindingAddr: bindEnv}, want: sbiaddr.DefaultBindHost},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(bindEnv, tt.env)
			sbi := tt.sbi
			ctx := initWithSbi(t, &sbi)
			if ctx.BindingIPv4 != tt.want {
				t.Errorf("BindingIPv4 = %q, want %q", ctx.BindingIPv4, tt.want)
			}
		})
	}
}

// TestGetSbiUriKeepsLegacyFormat checks that, for the RegisterIPv4 values that
// worked before (IPv4 addresses and host names), the own URI is the same as
// the one built with fmt.Sprintf("%s://%s:%d").
func TestGetSbiUriKeepsLegacyFormat(t *testing.T) {
	for _, host := range []string{"127.0.0.9", "1.1.1.1", "ausf", "ausf.sdcore.svc.cluster.local"} {
		for _, scheme := range []models.UriScheme{models.URISCHEME_HTTP, models.URISCHEME_HTTPS} {
			for _, port := range []int{80, 8000, 29509} {
				ctx := AUSFContext{UriScheme: scheme, RegisterIPv4: host, SBIPort: port}
				legacy := fmt.Sprintf("%s://%s:%d", scheme, host, port)
				if got := ctx.GetSbiUri(); got != legacy {
					t.Errorf("GetSbiUri() = %q, want %q", got, legacy)
				}
			}
		}
	}
}
