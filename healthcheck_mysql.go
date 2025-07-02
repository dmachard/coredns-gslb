package gslb

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// MySQLHealthCheck represents MySQL-specific health check settings.
type MySQLHealthCheck struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port" default:"3306"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	Timeout  string `yaml:"timeout" default:"3s"`
	Query    string `yaml:"query" default:"SELECT 1"`
}

func (h *MySQLHealthCheck) SetDefault() {
	if h.Port == 0 {
		h.Port = 3306
	}
	if h.Timeout == "" {
		h.Timeout = "3s"
	}
	if h.Query == "" {
		h.Query = "SELECT 1"
	}
}

func (h *MySQLHealthCheck) GetType() string {
	return fmt.Sprintf("mysql/%d", h.Port)
}

func (h *MySQLHealthCheck) PerformCheck(backend *Backend, fqdn string, maxRetries int) bool {
	h.SetDefault()
	typeStr := h.GetType()
	address := backend.Address
	start := time.Now()
	result := false
	defer func() {
		ObserveHealthcheck(typeStr, address, start, result)
	}()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=%s", h.User, h.Password, h.Host, h.Port, h.Database, h.Timeout)
	timeout, err := time.ParseDuration(h.Timeout)
	if err != nil {
		log.Errorf("[mysql] invalid timeout format: %v", err)
		return false
	}

	for retry := 0; retry <= maxRetries; retry++ {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Debugf("[mysql] connection failed: %v", err)
			if retry == maxRetries {
				return false
			}
			continue
		}
		db.SetConnMaxLifetime(timeout)
		db.SetConnMaxIdleTime(timeout)
		defer db.Close()

		// Ping to check connection
		pingErr := db.Ping()
		if pingErr != nil {
			log.Debugf("[mysql] ping failed: %v", pingErr)
			if retry == maxRetries {
				return false
			}
			continue
		}

		// Execute the query
		row := db.QueryRow(h.Query)
		var dummy int
		if err := row.Scan(&dummy); err != nil {
			log.Debugf("[mysql] query failed: %v", err)
			if retry == maxRetries {
				return false
			}
			continue
		}
		result = true
		return true
	}
	return false
}

func (h *MySQLHealthCheck) Equals(other GenericHealthCheck) bool {
	otherMySQL, ok := other.(*MySQLHealthCheck)
	if !ok {
		return false
	}
	return h.Host == otherMySQL.Host && h.Port == otherMySQL.Port && h.User == otherMySQL.User && h.Database == otherMySQL.Database && h.Query == otherMySQL.Query
}
