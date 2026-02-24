package app

import "time"

type ssoAccountsResponse struct {
	AccountList []struct {
		AccountID    string `json:"accountId"`
		AccountName  string `json:"accountName"`
		EmailAddress string `json:"emailAddress"`
	} `json:"accountList"`
}

type ssoRolesResponse struct {
	RoleList []struct {
		RoleName string `json:"roleName"`
	} `json:"roleList"`
}

type ec2DescribeInstancesResponse struct {
	Reservations []struct {
		Instances []struct {
			InstanceID      string `json:"InstanceId"`
			PrivateIP       string `json:"PrivateIpAddress"`
			PlatformDetails string `json:"PlatformDetails"`
			State           struct {
				Name string `json:"Name"`
			} `json:"State"`
			Tags []struct {
				Key   string `json:"Key"`
				Value string `json:"Value"`
			} `json:"Tags"`
		} `json:"Instances"`
	} `json:"Reservations"`
}

type ec2DescribeRegionsResponse struct {
	Regions []struct {
		RegionName string `json:"RegionName"`
	} `json:"Regions"`
}

type profileConfig struct {
	Name         string
	Region       string
	Output       string
	SSOSession   string
	SSOStartURL  string
	SSORegion    string
	SourceExists bool
}

type ssoCacheToken struct {
	StartURL    string `json:"startUrl"`
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
}

type roleTarget struct {
	AccountID   string
	AccountName string
	RoleName    string
}

type instanceCandidate struct {
	DisplayLine string
	ProfileName string
	Region      string
	InstanceID  string
}

type scanResult struct {
	Candidates []instanceCandidate
}

type Options struct {
	Profile              string
	Workers              int
	AccountFilter        string
	RoleFilter           string
	RoleFromPreferred    bool
	RegionsArg           string
	AllRegions           bool
	SkipRegionSelect     bool
	IncludeStopped       bool
	Resume               bool
	Last                 bool
	NoAutoSelect         bool
	ConfigPath           string
	WriteConfigExample   bool
	PrintEffectiveConfig bool
	FlagSet              map[string]bool
	CacheEnabled         bool
	CacheDir             string
	CacheTTLAccounts     time.Duration
	CacheTTLRoles        time.Duration
	CacheTTLRegions      time.Duration
	CacheTTLInstances    time.Duration
	CacheMode            string
	CacheClear           bool
	ValueSource          map[string]string
	cacheStore           *cacheStore
}
