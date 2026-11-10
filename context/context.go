// Copyright 2019 free5GC.org
// SPDX-FileCopyrightText: 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0
//

package context

import (
	"regexp"
	"sync"
	"time"

	"github.com/omec-project/ausf/logger"
	"github.com/omec-project/openapi/v2/models"
	"github.com/omec-project/util/sbiaddr"
)

type AUSFContext struct {
	suciSupiMap              sync.Map
	UePool                   sync.Map
	NfStatusSubscriptions    sync.Map // map[NfInstanceID]models.NrfSubscriptionData.SubscriptionId
	snRegex                  *regexp.Regexp
	NfId                     string
	GroupID                  string
	RegisterIPv4             string // IPv4 address or host name registered at the NRF
	RegisterIPv6             string // IPv6 address registered at the NRF
	BindingIPv4              string // SBI listen host; despite the name, of any family
	Url                      string
	NrfUri                   string
	UdmUeauUrl               string
	UriScheme                models.UriScheme
	Key                      string
	PEM                      string
	NfService                map[models.ServiceName]models.NFService
	PlmnList                 []models.PlmnId
	SBIPort                  int
	SbiPreference            sbiaddr.Preference
	EnableNrfCaching         bool
	NrfCacheEvictionInterval time.Duration
}

type AusfUeContext struct {
	Supi               string
	Kausf              string
	Kseaf              string
	ServingNetworkName string
	AuthStatus         models.AuthResult
	UdmUeauUrl         string

	// for 5G AKA
	XresStar string

	// for EAP-AKA'
	K_aut string
	XRES  string
	Rand  string
}

type SuciSupiMap struct {
	SupiOrSuci string
	Supi       string
}

// Attribute Types for EAP-AKA'
const (
	AT_RAND_ATTRIBUTE         = 1
	AT_AUTN_ATTRIBUTE         = 2
	AT_RES_ATTRIBUTE          = 3
	AT_MAC_ATTRIBUTE          = 11
	AT_NOTIFICATION_ATTRIBUTE = 12
	AT_IDENTITY_ATTRIBUTE     = 14
	AT_KDF_INPUT_ATTRIBUTE    = 23
	AT_KDF_ATTRIBUTE          = 24
)

// servingNetworkNamePattern is the ServingNetworkName pattern of TS 29.503 (Nudm_UEAU), fully anchored:
// "5G:mnc<3 digits>.mcc<3 digits>.3gppnetwork.org" with an optional ":<11 hex digits>" NPN suffix, or "5G:NSWO".
const servingNetworkNamePattern = `^(5G:mnc[0-9]{3}[.]mcc[0-9]{3}[.]3gppnetwork[.]org(:[A-F0-9]{11})?|5G:NSWO)$`

var ausfContext AUSFContext

func Init() {
	if snRegex, err := regexp.Compile(servingNetworkNamePattern); err != nil {
		logger.ContextLog.Warnf("SN compile error: %+v", err)
	} else {
		ausfContext.snRegex = snRegex
	}
	InitAusfContext(&ausfContext)
}

func NewAusfUeContext(identifier string) (ausfUeContext *AusfUeContext) {
	ausfUeContext = new(AusfUeContext)
	ausfUeContext.Supi = identifier // supi
	return ausfUeContext
}

func AddAusfUeContextToPool(ausfUeContext *AusfUeContext) {
	ausfContext.UePool.Store(ausfUeContext.Supi, ausfUeContext)
}

func RemoveAusfUeContextFromPool(supi string) {
	ausfContext.UePool.Delete(supi)
}

func CheckIfAusfUeContextExists(ref string) bool {
	_, ok := ausfContext.UePool.Load(ref)
	return ok
}

func GetAusfUeContext(ref string) *AusfUeContext {
	value, _ := ausfContext.UePool.Load(ref)
	ausfUeContext, _ := value.(*AusfUeContext)
	return ausfUeContext
}

// SbiAddrs classifies the configured register hosts (RegisterIPv4 and
// RegisterIPv6) into the FQDN, IPv4 and IPv6 addresses the AUSF advertises.
func (context *AUSFContext) SbiAddrs() sbiaddr.RegisterAddrs {
	addrs, err := sbiaddr.ParseRegister(context.RegisterIPv4, context.RegisterIPv6)
	if err != nil {
		// InitAusfContext has already warned about the configured values.
		logger.ContextLog.Debugf("SBI register address: %v", err)
	}
	return addrs
}

// GetSbiUri is the AUSF's own base URI, scheme://host:port, with an IPv6 host
// in brackets.
func (context *AUSFContext) GetSbiUri() string {
	return context.SbiAddrs().URI(string(context.UriScheme), context.SBIPort, context.SbiPreference)
}

// GetIPv4Uri is the former name of GetSbiUri, kept for existing callers.
// Despite the name, the host may be an IPv6 address.
func (context *AUSFContext) GetIPv4Uri() string {
	return context.GetSbiUri()
}

func AddSuciSupiPairToMap(supiOrSuci string, supi string) {
	newPair := new(SuciSupiMap)
	newPair.SupiOrSuci = supiOrSuci
	newPair.Supi = supi
	ausfContext.suciSupiMap.Store(supiOrSuci, newPair)
}

func RemoveSuciSupiPairFromMap(supiOrSuci string) {
	ausfContext.suciSupiMap.Delete(supiOrSuci)
}

func ListSuciSupiPairsForSupi(supi string) []string {
	keys := make([]string, 0)
	ausfContext.suciSupiMap.Range(func(key, value any) bool {
		pair, ok := value.(*SuciSupiMap)
		if !ok || pair == nil || pair.Supi != supi {
			return true
		}
		if supiOrSuci, ok := key.(string); ok {
			keys = append(keys, supiOrSuci)
		}
		return true
	})
	return keys
}

func HasSuciSupiPairForSupi(supi string) bool {
	return len(ListSuciSupiPairsForSupi(supi)) > 0
}

func CheckIfSuciSupiPairExists(ref string) bool {
	_, ok := ausfContext.suciSupiMap.Load(ref)
	return ok
}

func GetSupiFromSuciSupiMap(ref string) (supi string) {
	val, _ := ausfContext.suciSupiMap.Load(ref)
	suciSupiMap, ok := val.(*SuciSupiMap)
	if !ok || suciSupiMap == nil {
		return ""
	}
	return suciSupiMap.Supi
}

func IsServingNetworkAuthorized(lookup string) bool {
	// fail closed if the pattern failed to compile in Init
	if ausfContext.snRegex == nil {
		return false
	}
	return ausfContext.snRegex.MatchString(lookup)
}

func GetSelf() *AUSFContext {
	return &ausfContext
}

func (context *AUSFContext) GetSelfID() string {
	return context.NfId
}
