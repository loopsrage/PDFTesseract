package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/loopsrage/go_mltools/proceduralphasespace/ppsclient"
	"github.com/loopsrage/gotools/lib/docpipeline"
	"github.com/loopsrage/gotools/lib/ocrlib"
	"github.com/loopsrage/gotools/lib/reflectStruct"
	"github.com/loopsrage/gotools/lib/settings"
)

// Config holds HTTP server socket and timeout parameters.
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
	if err := json.Unmarshal(i, &x); err != nil {
		return err
	}
	return reflectStruct.CopyToStructFromMap(h, x)
}

// Type aliases re-exporting library configuration structs from go_mltools and gotools.
type (
	PhaseServerConfig       = ppsclient.Config
	StageTuning             = docpipeline.StageTuning
	PipelineConfig          = docpipeline.PipelineConfig
	TesseractProviderConfig = ocrlib.TesseractProviderConfig
	HTTPProviderConfig      = ocrlib.HTTPProviderConfig
	OllamaProviderConfig    = ocrlib.OllamaProviderConfig
	MLToolsProviderConfig   = ocrlib.MLToolsProviderConfig
	OCRConfig               = ocrlib.Config
)

// AppConfig aggregates full service settings with defaults and env overrides.
type AppConfig struct {
	Server      Config
	PhaseServer PhaseServerConfig
	Pipeline    PipelineConfig
	OCR         OCRConfig
}

// EngineConfig projects AppConfig into a docpipeline.Config.
func (a AppConfig) EngineConfig() docpipeline.Config {
	return docpipeline.Config{
		PhaseServer: a.PhaseServer,
		Pipeline:    a.Pipeline,
		OCR:         a.OCR,
	}
}

// DefaultAppConfig returns canonical defaults targeting Minikube proceduralphaseserver (:30180).
func DefaultAppConfig() AppConfig {
	base := docpipeline.DefaultConfig()
	base.PhaseServer.BaseURL = "http://127.0.0.1:30180"
	base.PhaseServer.Collection = "ocr_artifacts"
	base.PhaseServer.RootTopic = "ocr.documents"
	base.PhaseServer.ServiceName = "pdftesseract"
	base.PhaseServer.ServiceEmail = "pdftesseract@gmail.com"
	base.PhaseServer.LogTopic = "cluster.logs.pdftesseract"
	base.Pipeline.ServiceName = "pdftesseract"

	return AppConfig{
		Server: Config{
			Addr:           "0.0.0.0:8081",
			MaxHeaderBytes: 10485760,
		},
		PhaseServer: base.PhaseServer,
		Pipeline:    base.Pipeline,
		OCR:         base.OCR,
	}
}

// LoadAppConfig merges defaults, loaded settings (env.yaml), and explicit environment variables.
func LoadAppConfig() AppConfig {
	cfg := DefaultAppConfig()

	_ = settings.JsonUnmarshalSettingsTo("HttpServer", &cfg.Server)
	_ = settings.JsonUnmarshalSettingsTo("PhaseServer", &cfg.PhaseServer)
	_ = settings.JsonUnmarshalSettingsTo("Pipeline", &cfg.Pipeline)
	_ = settings.JsonUnmarshalSettingsTo("OCR", &cfg.OCR)

	if v := strings.TrimSpace(os.Getenv("PPS_BASE_URL")); v != "" {
		cfg.PhaseServer.BaseURL = v
	}
	if v := strings.TrimSpace(os.Getenv("PPS_OAUTH_TOKEN")); v != "" {
		cfg.PhaseServer.OAuthToken = v
	}
	if v := strings.TrimSpace(os.Getenv("PPS_SERVICE_EMAIL")); v != "" {
		cfg.PhaseServer.ServiceEmail = v
	}
	if v := strings.TrimSpace(os.Getenv("PPS_COLLECTION")); v != "" {
		cfg.PhaseServer.Collection = v
	}
	if v := strings.TrimSpace(os.Getenv("PPS_ROOT_TOPIC")); v != "" {
		cfg.PhaseServer.RootTopic = v
	}
	if v := strings.TrimSpace(os.Getenv("OCR_DEFAULT_PROVIDER")); v != "" {
		cfg.OCR.DefaultProvider = v
	}
	if v := strings.TrimSpace(os.Getenv("OCR_HTTP_ENDPOINT")); v != "" {
		cfg.OCR.HTTP.Endpoint = v
		cfg.OCR.HTTP.Enabled = true
	}
	if v := strings.TrimSpace(os.Getenv("OCR_OLLAMA_MODEL")); v != "" {
		cfg.OCR.Ollama.Model = v
		cfg.OCR.Ollama.Enabled = true
	}
	if v := strings.TrimSpace(os.Getenv("TESSERACT_BIN")); v != "" {
		cfg.OCR.Tesseract.BinaryPath = v
	}
	if v := strings.TrimSpace(os.Getenv("TESSDATA_PREFIX")); v != "" && cfg.OCR.Tesseract.TessdataDir == "" {
		cfg.OCR.Tesseract.TessdataDir = v
	}
	if v := strings.TrimSpace(os.Getenv("HTTP_ADDR")); v != "" {
		cfg.Server.Addr = v
	}
	if v := strings.TrimSpace(os.Getenv("PPS_CHUNK_BYTES")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.PhaseServer.ChunkBytes = n
		}
	}

	if cfg.PhaseServer.ChunkBytes <= 0 {
		cfg.PhaseServer.ChunkBytes = 4 * 1024 * 1024
	}
	return cfg
}
