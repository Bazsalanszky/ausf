// SPDX-FileCopyrightText: 2026 Balázs Toldi
//
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"testing"

	"github.com/omec-project/ausf/consumer"
	ausf_context "github.com/omec-project/ausf/context"
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/util/sbiaddr"
)

const (
	udmTestIPv4  = "10.0.0.7"
	udmTestIPv6  = "fd00::7"
	udmTestV6URI = "https://[fd00::7]:29503"
)

func udmProfile(fqdn string, ipv6s []string, services ...models.NFService) models.NFProfileDiscovery {
	return models.NFProfileDiscovery{
		NfInstanceId:  "udm-1",
		NfType:        models.NFTYPE_UDM,
		NfStatus:      models.NFSTATUS_REGISTERED,
		Fqdn:          nilIfEmpty(fqdn),
		Ipv6Addresses: ipv6s,
		NfServices:    services,
	}
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ueauService(endPoints ...models.IpEndPoint) models.NFService {
	return models.NFService{
		ServiceInstanceId: "ueau-1",
		ServiceName:       models.SERVICENAME_NUDM_UEAU,
		Scheme:            models.URISCHEME_HTTPS,
		NfServiceStatus:   models.NFSERVICESTATUS_REGISTERED,
		IpEndPoints:       endPoints,
	}
}

func v4EndPoint(addr string, port int32) models.IpEndPoint {
	return models.IpEndPoint{Ipv4Address: &addr, Port: &port}
}

func v6EndPoint(addr string, port int32) models.IpEndPoint {
	return models.IpEndPoint{Ipv6Address: &addr, Port: &port}
}

func TestGetUdmUrlAddressFamilies(t *testing.T) {
	tests := []struct {
		name       string
		profile    models.NFProfileDiscovery
		preference sbiaddr.Preference
		want       string
	}{
		{
			name:    "IPv4 endpoint",
			profile: udmProfile("", nil, ueauService(v4EndPoint(udmTestIPv4, 29503))),
			want:    "https://10.0.0.7:29503",
		},
		{
			name:    "IPv6-only endpoint is bracketed",
			profile: udmProfile("", []string{udmTestIPv6}, ueauService(v6EndPoint(udmTestIPv6, 29503))),
			want:    udmTestV6URI,
		},
		{
			name: "dual-stack endpoints, IPv4 preferred",
			profile: udmProfile("", nil,
				ueauService(v4EndPoint(udmTestIPv4, 29503), v6EndPoint(udmTestIPv6, 29503))),
			want: "https://10.0.0.7:29503",
		},
		{
			name: "dual-stack endpoints, IPv6 preferred",
			profile: udmProfile("", nil,
				ueauService(v4EndPoint(udmTestIPv4, 29503), v6EndPoint(udmTestIPv6, 29503))),
			preference: sbiaddr.PreferIPv6,
			want:       udmTestV6URI,
		},
		{
			name:    "profile FQDN",
			profile: udmProfile("udm", nil, ueauService(models.IpEndPoint{Port: new(int32(29503))})),
			want:    "https://udm:29503",
		},
		{
			name:    "endpoint without a port gets the scheme default",
			profile: udmProfile("", nil, ueauService(models.IpEndPoint{Ipv4Address: new(udmTestIPv4)})),
			want:    "https://10.0.0.7:443",
		},
		{
			name: "endpoint without an address falls back to the profile IPv6 address",
			profile: udmProfile("", []string{udmTestIPv6},
				ueauService(models.IpEndPoint{Port: new(int32(29503))})),
			want: udmTestV6URI,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			self := ausf_context.GetSelf()
			origSearch, origDirect, origPref := consumer.SendSearchNFInstances, consumer.SendNfDiscoveryToNrf, self.SbiPreference
			t.Cleanup(func() {
				consumer.SendSearchNFInstances, consumer.SendNfDiscoveryToNrf = origSearch, origDirect
				self.SbiPreference = origPref
				invalidateUdmCache()
			})
			consumer.SendSearchNFInstances = func(string, models.NFType, models.NFType,
				consumer.SearchNFInstancesRequestConfigurer,
			) (*models.SearchResult, error) {
				return models.NewSearchResult(1, []models.NFProfileDiscovery{tt.profile}), nil
			}
			consumer.SendNfDiscoveryToNrf = func(context.Context, string, models.NFType, models.NFType,
				consumer.SearchNFInstancesRequestConfigurer,
			) (*models.SearchResult, error) {
				t.Fatal("did not expect direct NRF discovery fallback")
				return nil, nil
			}
			invalidateUdmCache()
			self.SbiPreference = tt.preference

			if got := GetUdmUrl(""); got != tt.want {
				t.Errorf("GetUdmUrl() = %q, want %q", got, tt.want)
			}
		})
	}
}
