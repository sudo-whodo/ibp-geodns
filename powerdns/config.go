package powerdns

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

func loadStaticEntries(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch static entries: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var entries []Record
	if err := json.Unmarshal(body, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal static entries: %w", err)
	}

	newStaticEntries := make(map[string][]Record)
	for _, entry := range entries {
		entry.Qname = strings.TrimSuffix(entry.Qname, ".")
		newStaticEntries[entry.Qname] = append(newStaticEntries[entry.Qname], entry)
	}

	staticEntries = newStaticEntries

	return nil
}

func updateStaticEntries(staticEntriesURL string) {
	mu.Lock()
	defer mu.Unlock()

	err := loadStaticEntries(staticEntriesURL)
	if err != nil {
		log.Printf("Failed to update static entries: %v", err)
	} else {
		log.Println("Static entries updated successfully")
	}
}

func startStaticEntriesUpdater(staticEntriesURL string) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		<-ticker.C
		updateStaticEntries(staticEntriesURL)
	}
}
