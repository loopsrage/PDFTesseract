package config

import (
	"Tesseract/lib/reflectStruct"
	"encoding/json"
	"time"
)

type Config struct {
	Addr              string        `json:"Addr"`
	ReadTimeout       time.Duration `json:"ReadTimeout"`
	ReadHeaderTimeout time.Duration `json:"ReadHeaderTimeout"`
	WriteTimeout      time.Duration `json:"WriteTimeout"`
	IdleTimeout       time.Duration `json:"IdleTimeout"`
	MaxHeaderBytes    int           `json:"MaxHeaderBytes"`
}

func (h *Config) UnmarshalJSON(i []byte) error {
	x := make(map[string]interface{})
	err := json.Unmarshal(i, &x)
	if err != nil {
		return err
	}
	return reflectStruct.CopyToStructFromMap(h, x)
}
