package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/nxadm/tail"
	"golang.org/x/time/rate"
	"gopkg.in/yaml.v3"
)

type Config struct {
	DestinationURL       string   `yaml:"destination_url"`
	AuthKey              string   `yaml:"auth_key"`
	LogPaths             []string `yaml:"log_paths"`
	DropRules            []string `yaml:"drop_rules"`
	BatchSize            int      `yaml:"batch_size"`
	FlushIntervalSeconds int      `yaml:"flush_interval_seconds"`
	MaxMBPerSecond       int      `yaml:"max_mb_per_second"`
}

type LogEvent struct {
	Timestamp string `json:"timestamp"`
	Host      string `json:"host"`
	Source    string `json:"source"`
	Message   string `json:"message"`
}

func main() {
	fmt.Println("Starting Log Forwarder Agent...")

	config := loadConfig("config.yaml")
	hostname, _ := os.Hostname()

	var compiledDropRules []*regexp.Regexp
	for _, rule := range config.DropRules {
		compiledDropRules = append(compiledDropRules, regexp.MustCompile(rule))
	}

	logChan := make(chan LogEvent, config.BatchSize*5)
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	go batchProcessor(logChan, config)

	bytesPerSec := config.MaxMBPerSecond * 1024 * 1024
	limiter := rate.NewLimiter(rate.Limit(bytesPerSec), bytesPerSec)

	var activeTailers []*tail.Tail
	for _, pattern := range config.LogPaths {
		files, err := filepath.Glob(pattern)
		if err != nil {
			log.Printf("Invalid glob pattern %s: %v", pattern, err)
			continue
		}
		for _, file := range files {
			fmt.Printf("Tailing discovered file: %s\n", file)
			t, err := tail.TailFile(file, tail.Config{
				Follow: true, ReOpen: true, MustExist: false,
				Location: &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END},
			})
			if err != nil {
				log.Printf("Failed to tail %s: %v", file, err)
				continue
			}
			activeTailers = append(activeTailers, t)
			go processFile(t, file, hostname, logChan, compiledDropRules, limiter)
		}
	}

	<-shutdownChan
	fmt.Println("\nShutdown signal received. Stopping tailers and flushing logs...")
	for _, t := range activeTailers {
		t.Stop()
	}
	close(logChan)
	time.Sleep(3 * time.Second)
	os.Exit(0)
}

func processFile(t *tail.Tail, filename, hostname string, logChan chan<- LogEvent, dropRules []*regexp.Regexp, limiter *rate.Limiter) {
	for line := range t.Lines {
		if line == nil || line.Text == "" {
			continue
		}

		limiter.WaitN(context.Background(), len(line.Text))

		drop := false
		for _, rule := range dropRules {
			if rule.MatchString(line.Text) {
				drop = true
				break
			}
		}
		if drop {
			continue
		}

		logChan <- LogEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Host:      hostname,
			Source:    filename,
			Message:   line.Text,
		}
	}
}

func batchProcessor(logChan <-chan LogEvent, config Config) {
	var batch []LogEvent
	ticker := time.NewTicker(time.Duration(config.FlushIntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-logChan:
			if !ok {
				if len(batch) > 0 {
					sendPayload(batch, config)
				}
				return
			}
			batch = append(batch, event)
			if len(batch) >= config.BatchSize {
				sendPayload(batch, config)
				batch = nil
			}
		case <-ticker.C:
			if len(batch) > 0 {
				sendPayload(batch, config)
				batch = nil
			}
		}
	}
}

func sendPayload(batch []LogEvent, config Config) {
	jsonData, _ := json.Marshal(batch)
	req, _ := http.NewRequest("POST", config.DestinationURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	if config.AuthKey != "" {
		req.Header.Set("Authorization", "Bearer "+config.AuthKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		log.Printf("Flushed %d logs successfully.", len(batch))
	} else {
		log.Printf("Failed to send batch: %v", err)
	}
}

func loadConfig(filename string) Config {
	data, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Error reading config: %v", err)
	}
	var config Config
	yaml.Unmarshal(data, &config)
	return config
}