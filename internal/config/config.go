package config

import (
	"encoding/json"
	"os"
	"sync"
)

const ConfigFile = "config.json"

type RouterConfig struct {
	IP       string `json:"ip"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"` // hotspot display name
}

type AppConfig struct {
	mu      sync.RWMutex
	Routers []RouterConfig `json:"routers"`
	Admin   struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"admin"`
	Theme    string `json:"theme"`    // "dark" | "light"
	Currency string `json:"currency"` // e.g. "Rp"
	Timezone string `json:"timezone"` // e.g. "Asia/Jakarta"
	Language string `json:"language"` // "id" | "en"
	Secret   string `json:"secret"`   // session secret key
}

var (
	instance *AppConfig
	once     sync.Once
)

func Load() (*AppConfig, error) {
	var err error
	once.Do(func() {
		instance = &AppConfig{
			Theme:    "dark",
			Currency: "Rp",
			Timezone: "Asia/Jakarta",
			Language: "id",
			Secret:   "mikvoc-secret-change-me",
		}
		instance.Admin.Username = "admin"
		instance.Admin.Password = "admin"

		f, e := os.ReadFile(ConfigFile)
		if e != nil {
			// First run: write defaults
			_ = instance.Save()
			return
		}
		err = json.Unmarshal(f, instance)
	})
	return instance, err
}

func Get() *AppConfig {
	return instance
}

func (c *AppConfig) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigFile, b, 0600)
}

func (c *AppConfig) ActiveRouter() *RouterConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.Routers) == 0 {
		return nil
	}
	return &c.Routers[0]
}
