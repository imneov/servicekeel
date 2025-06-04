package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// parseResolvConf reads and parses /etc/resolv.conf file
func parseResolvConf() ([]string, error) {
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return nil, fmt.Errorf("failed to open resolv.conf: %w", err)
	}
	defer file.Close()

	var searchDomains []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "search") {
			// 解析 search 行
			fields := strings.Fields(line)
			if len(fields) > 1 {
				searchDomains = append(searchDomains, fields[1:]...)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading resolv.conf: %w", err)
	}

	return searchDomains, nil
}
