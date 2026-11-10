// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"slices"
	"testing"

	ausfctx "github.com/omec-project/ausf/context"
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/util/sbiaddr"
)

const (
	testNfID  = "test-nf-id"
	testIPv4  = "10.0.0.5"
	testIPv6  = "fd00::5"
	testFQDN  = "ausf"
	testV6URI = "https://[fd00::5]:29509"
)

func TestGetNfProfileAddresses(t *testing.T) {
	tests := []struct {
		name         string
		registerIPv4 string
		registerIPv6 string
		wantFqdn     string
		wantIPv4     []string
		wantIPv6     []string
	}{
		{name: "IPv4", registerIPv4: testIPv4, wantIPv4: []string{testIPv4}},
		{name: "FQDN", registerIPv4: testFQDN, wantFqdn: testFQDN},
		{name: "IPv6 in RegisterIPv4", registerIPv4: testIPv6, wantIPv6: []string{testIPv6}},
		{name: "IPv6", registerIPv6: testIPv6, wantIPv6: []string{testIPv6}},
		{
			name: "dual stack", registerIPv4: testIPv4, registerIPv6: testIPv6,
			wantIPv4: []string{testIPv4}, wantIPv6: []string{testIPv6},
		},
		{name: "FQDN and IPv6", registerIPv4: testFQDN, registerIPv6: testIPv6, wantFqdn: testFQDN, wantIPv6: []string{testIPv6}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &ausfctx.AUSFContext{NfId: testNfID, RegisterIPv4: tt.registerIPv4, RegisterIPv6: tt.registerIPv6}
			profile, err := getNfProfile(ctx, nil)
			if err != nil {
				t.Fatalf("getNfProfile: %v", err)
			}
			if got := profile.GetFqdn(); got != tt.wantFqdn {
				t.Errorf("fqdn = %q, want %q", got, tt.wantFqdn)
			}
			if got := profile.GetIpv4Addresses(); !slices.Equal(got, tt.wantIPv4) {
				t.Errorf("ipv4Addresses = %q, want %q", got, tt.wantIPv4)
			}
			if got := profile.GetIpv6Addresses(); !slices.Equal(got, tt.wantIPv6) {
				t.Errorf("ipv6Addresses = %q, want %q", got, tt.wantIPv6)
			}
		})
	}
}

func TestGetNfProfile_WhenRegisterAddressIsEmpty_ThenReturnsError(t *testing.T) {
	for _, registerIPv4 := range []string{"", "not a host"} {
		ctx := &ausfctx.AUSFContext{NfId: testNfID, RegisterIPv4: registerIPv4}
		if _, err := getNfProfile(ctx, nil); err == nil {
			t.Errorf("RegisterIPv4 %q: expected an error, got nil", registerIPv4)
		}
	}
}

func TestBuildSubscriptionData_NotificationUriBracketsIPv6(t *testing.T) {
	tests := []struct {
		name         string
		registerIPv4 string
		registerIPv6 string
		wantPrefix   string
		preference   sbiaddr.Preference
	}{
		{name: "IPv6", registerIPv6: testIPv6, wantPrefix: testV6URI},
		{
			name: "dual stack, IPv4 preferred", registerIPv4: testIPv4, registerIPv6: testIPv6,
			wantPrefix: "https://10.0.0.5:29509",
		},
		{
			name: "dual stack, IPv6 preferred", registerIPv4: testIPv4, registerIPv6: testIPv6,
			preference: sbiaddr.PreferIPv6, wantPrefix: testV6URI,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &ausfctx.AUSFContext{
				UriScheme:     models.URISCHEME_HTTPS,
				RegisterIPv4:  tt.registerIPv4,
				RegisterIPv6:  tt.registerIPv6,
				SBIPort:       29509,
				SbiPreference: tt.preference,
			}
			got := buildSubscriptionData(ctx, "test-nf-instance-id", models.NFTYPE_AMF).GetNfStatusNotificationUri()
			if want := tt.wantPrefix + "/nausf-callback/v1/nf-status-notify"; got != want {
				t.Errorf("NfStatusNotificationUri = %q, want %q", got, want)
			}
		})
	}
}
