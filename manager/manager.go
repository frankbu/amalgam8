// Copyright 2016 IBM Corporation
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

package manager

import (
	"net/http"

	"time"

	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/amalgam8/controller/database"
	"github.com/amalgam8/controller/notification"
	"github.com/amalgam8/controller/resources"
)

// Manager client
type Manager interface {
	Create(id string, rules resources.TenantInfo) error
	Set(id string, rules resources.TenantInfo) error
	Get(id string) (resources.TenantEntry, error)
	Delete(id string) error
	SetVersion(id string, version resources.Version) error
	DeleteVersion(id, service string) error
	GetVersion(id, service string) (resources.Version, error)
}

type manager struct {
	db            database.Tenant
	producerCache notification.TenantProducerCache
}

// Config options
type Config struct {
	Database      database.Tenant
	ProducerCache notification.TenantProducerCache
}

// NewManager creates Manager instance
func NewManager(conf Config) Manager {
	return &manager{
		db:            conf.Database,
		producerCache: conf.ProducerCache,
	}
}

func (m *manager) Create(id string, tenantInfo resources.TenantInfo) error {
	var err error

	entry := resources.TenantEntry{
		BasicEntry: resources.BasicEntry{
			ID: id,
		},
		ProxyConfig: resources.ProxyConfig{
			LoadBalance:       tenantInfo.LoadBalance,
			Port:              tenantInfo.Port,
			ReqTrackingHeader: tenantInfo.ReqTrackingHeader,
			Credentials: resources.Credentials{
				Kafka:    tenantInfo.Credentials.Kafka,
				Registry: tenantInfo.Credentials.Registry,
			},
			Filters: tenantInfo.Filters,
		},
		ServiceCatalog: resources.ServiceCatalog{
			Services:   []resources.Service{},
			LastUpdate: time.Now(),
		},
	}

	// Copy each element

	if entry.ProxyConfig.Filters.Rules == nil {
		entry.ProxyConfig.Filters.Rules = []resources.Rule{}
	}

	if entry.ProxyConfig.Filters.Versions == nil {
		entry.ProxyConfig.Filters.Versions = []resources.Version{}
	}

	// Set defaults if necessary
	if entry.ProxyConfig.LoadBalance == "" {
		entry.ProxyConfig.LoadBalance = "round_robin" // FIXME: common location for this?
	}

	if entry.ProxyConfig.Port == 0 {
		entry.ProxyConfig.Port = 6379 // FIXME
	}

	if entry.ProxyConfig.ReqTrackingHeader == "" {
		entry.ProxyConfig.ReqTrackingHeader = "X-Request-ID" // FIXME: common location for this?
	}

	if err = validateRules(entry.ProxyConfig.Filters.Rules); err != nil {
		// RestError() called in validate function
		return err
	}

	rules := []resources.Rule{}
	for _, rule := range entry.ProxyConfig.Filters.Rules {
		if rule.DelayProbability == 0.0 && rule.AbortProbability == 0.0 {
			continue
		}
		rules = append(rules, rule)
	}
	entry.ProxyConfig.Filters.Rules = rules

	// Ensure Registry credentials are provided
	if entry.ProxyConfig.Credentials.Registry.URL == "" || entry.ProxyConfig.Credentials.Registry.Token == "" {
		return &InvalidRuleError{Reason: "must provide Registry creds", ErrorMessage: "invalid_registry_creds"}
	}

	mhCredValid := false

	if !entry.ProxyConfig.Credentials.Kafka.SASL && len(entry.ProxyConfig.Credentials.Kafka.Brokers) != 0 &&
		entry.ProxyConfig.Credentials.Kafka.APIKey == "" &&
		entry.ProxyConfig.Credentials.Kafka.AdminURL == "" &&
		entry.ProxyConfig.Credentials.Kafka.RestURL == "" &&
		entry.ProxyConfig.Credentials.Kafka.Password == "" &&
		entry.ProxyConfig.Credentials.Kafka.User == "" {

		// local kafka case
		mhCredValid = true
	} else if entry.ProxyConfig.Credentials.Kafka.SASL && entry.ProxyConfig.Credentials.Kafka.APIKey != "" &&
		entry.ProxyConfig.Credentials.Kafka.AdminURL != "" &&
		len(entry.ProxyConfig.Credentials.Kafka.Brokers) != 0 &&
		entry.ProxyConfig.Credentials.Kafka.RestURL != "" &&
		entry.ProxyConfig.Credentials.Kafka.Password != "" &&
		entry.ProxyConfig.Credentials.Kafka.User != "" {

		// Bluemix Message Hub case
		mhCredValid = true
	} else if !entry.ProxyConfig.Credentials.Kafka.SASL && len(entry.ProxyConfig.Credentials.Kafka.Brokers) == 0 &&
		entry.ProxyConfig.Credentials.Kafka.APIKey == "" &&
		entry.ProxyConfig.Credentials.Kafka.AdminURL == "" &&
		entry.ProxyConfig.Credentials.Kafka.RestURL == "" &&
		entry.ProxyConfig.Credentials.Kafka.Password == "" &&
		entry.ProxyConfig.Credentials.Kafka.User == "" {

		// no kafka messaging used
		mhCredValid = true
	}

	if !mhCredValid {
		return &InvalidRuleError{Reason: "must provide all Kafka creds", ErrorMessage: "invalid_kafka_creds"}
	}

	// TODO: perform a check to ensure that the SD and MH credentials actually work?

	// Add to rules
	if err = m.db.Create(entry); err != nil {
		logrus.WithError(err).Error("Failed setting rules")
		return &ServiceUnavailableError{Reason: "Could not create entry", ErrorMessage: "FIXME", Err: err}
	}

	// Send Kafka event
	if err = m.producerCache.SendEvent(entry.ID, entry.ProxyConfig.Credentials.Kafka); err != nil {
		return err
	}

	return nil
}

// Set database entry
func (m *manager) Set(id string, tenantInfo resources.TenantInfo) error {
	var err error

	setRegistry := false
	setKafka := false

	if tenantInfo.Credentials.Registry.URL != "" && tenantInfo.Credentials.Registry.Token != "" {
		setRegistry = true
	} else if tenantInfo.Credentials.Registry.URL != "" || tenantInfo.Credentials.Registry.Token != "" {
		return &InvalidRuleError{Reason: "bad Registry credentials", ErrorMessage: "bad_registry_creds"}
	}

	if tenantInfo.Credentials.Kafka.APIKey != "" &&
		tenantInfo.Credentials.Kafka.AdminURL != "" &&
		len(tenantInfo.Credentials.Kafka.Brokers) != 0 &&
		tenantInfo.Credentials.Kafka.RestURL != "" &&
		tenantInfo.Credentials.Kafka.Password != "" &&
		tenantInfo.Credentials.Kafka.User != "" {
		setKafka = true
	} else if tenantInfo.Credentials.Kafka.APIKey == "" &&
		tenantInfo.Credentials.Kafka.AdminURL == "" &&
		len(tenantInfo.Credentials.Kafka.Brokers) != 0 &&
		tenantInfo.Credentials.Kafka.RestURL == "" &&
		tenantInfo.Credentials.Kafka.User == "" &&
		tenantInfo.Credentials.Kafka.Password == "" &&
		!tenantInfo.Credentials.Kafka.SASL {
		setKafka = true
	} else if tenantInfo.Credentials.Kafka.APIKey != "" ||
		tenantInfo.Credentials.Kafka.AdminURL != "" ||
		len(tenantInfo.Credentials.Kafka.Brokers) != 0 ||
		tenantInfo.Credentials.Kafka.RestURL != "" ||
		tenantInfo.Credentials.Kafka.Password != "" ||
		tenantInfo.Credentials.Kafka.User != "" {
		return &InvalidRuleError{Reason: "bad Kafka credentials", ErrorMessage: "bad_kafka_creds"}
	}

	// TODO: only read and set proxyconfig if necessary
	entry, err := m.db.Read(id)
	if err != nil {
		//handleDBError(w, req, err)
		return &DBError{Err: err}
	}

	if setRegistry || setKafka {
		// TODO: perform a check to ensure that the Registry and Kafka credentials actually work?

		if setRegistry {
			entry.ProxyConfig.Credentials.Registry = tenantInfo.Credentials.Registry
		}

		if setKafka {
			entry.ProxyConfig.Credentials.Kafka = tenantInfo.Credentials.Kafka
		}
	}

	if tenantInfo.LoadBalance != "" {
		entry.ProxyConfig.LoadBalance = tenantInfo.LoadBalance
	}

	if tenantInfo.Port > 0 {
		entry.ProxyConfig.Port = tenantInfo.Port
	}

	if tenantInfo.ReqTrackingHeader != "" {
		entry.ProxyConfig.ReqTrackingHeader = tenantInfo.ReqTrackingHeader
	}

	if tenantInfo.Filters.Rules != nil {
		if err = validateRules(tenantInfo.Filters.Rules); err != nil {
			return err
		}

		rules := []resources.Rule{}
		for _, rule := range tenantInfo.Filters.Rules {
			if rule.DelayProbability == 0.0 && rule.AbortProbability == 0.0 {
				continue
			}
			rules = append(rules, rule)
		}
		entry.ProxyConfig.Filters.Rules = rules
	}

	if tenantInfo.Filters.Versions != nil {
		//TODO validate fields
		entry.ProxyConfig.Filters.Versions = tenantInfo.Filters.Versions
	}

	if err = m.updateProxyConfig(entry); err != nil {
		logrus.WithFields(logrus.Fields{
			"err":       err,
			"tenant_id": id,
			//"request_id": reqID,
		}).Error("Error updating info for tenant ID")
		return &ServiceUnavailableError{Reason: "database update failed", ErrorMessage: "database_fail", Err: err}
	}

	// Send Kafka event
	if err = m.producerCache.SendEvent(entry.ID, entry.ProxyConfig.Credentials.Kafka); err != nil {
		return err
	}

	return nil
}

// Get database entry
func (m *manager) Get(id string) (resources.TenantEntry, error) {
	entry, err := m.db.Read(id)
	if err != nil {
		return entry, &DBError{Err: err}
	}
	return entry, nil
}

// Delete database entry
func (m *manager) Delete(id string) error {
	if err := m.db.Delete(id); err != nil {
		return &DBError{Err: err}
	}
	return nil
}

func (m *manager) SetVersion(id string, newVersion resources.Version) error {
	entry, err := m.db.Read(id)
	if err != nil {
		return &DBError{Err: err}

	}

	updateIndex := -1
	for index, version := range entry.ProxyConfig.Filters.Versions {
		if version.Service == newVersion.Service {
			updateIndex = index
			break
		}
	}
	if updateIndex == -1 {
		entry.ProxyConfig.Filters.Versions = append(entry.ProxyConfig.Filters.Versions, newVersion)
	} else {
		entry.ProxyConfig.Filters.Versions[updateIndex] = newVersion
	}

	// Update the entry in the database
	if err = m.updateProxyConfig(entry); err != nil {

		logrus.WithFields(logrus.Fields{
			"err":       err,
			"tenant_id": id,
			//"request_id": reqID,
		}).Error("Error updating info for tenant ID")
		return &ServiceUnavailableError{Reason: "database update failed", ErrorMessage: "database_fail", Err: err}
	}

	// Send Kafka event
	if err = m.producerCache.SendEvent(entry.ID, entry.ProxyConfig.Credentials.Kafka); err != nil {
		return err
	}

	return nil
}

func (m *manager) DeleteVersion(id, service string) error {

	entry, err := m.db.Read(id)
	if err != nil {
		return &DBError{Err: err}

	}

	updateIndex := -1
	for index, version := range entry.ProxyConfig.Filters.Versions {
		if version.Service == service {
			updateIndex = index
			break
		}
	}
	if updateIndex == -1 {
		logrus.Error(fmt.Sprintf("No registered service(s) for %v matching service name %v", id, service))
		return &RuleNotFoundError{Reason: "No registered service(s) matching name", ErrorMessage: "invalid_service"}
	}

	entry.ProxyConfig.Filters.Versions = append(entry.ProxyConfig.Filters.Versions[:updateIndex], entry.ProxyConfig.Filters.Versions[updateIndex+1:]...)

	// Update the entry in the database
	if err = m.updateProxyConfig(entry); err != nil {
		logrus.WithFields(logrus.Fields{
			"err":       err,
			"tenant_id": id,
			//"request_id": reqID,
		}).Error("Error updating info for tenant ID")
		return &ServiceUnavailableError{Reason: "database update failed", ErrorMessage: "database_fail", Err: err}
	}

	// Send Kafka event
	if err = m.producerCache.SendEvent(entry.ID, entry.ProxyConfig.Credentials.Kafka); err != nil {
		return err
	}

	return nil
}

func (m *manager) GetVersion(id, service string) (resources.Version, error) {
	entry, err := m.db.Read(id)
	if err != nil {
		return resources.Version{}, &DBError{Err: err}

	}

	for _, version := range entry.ProxyConfig.Filters.Versions {
		if version.Service == service {
			return version, nil
		}
	}

	logrus.Error(fmt.Sprintf("No registered service(s) for %v matching service name %v", id, service))
	return resources.Version{}, &RuleNotFoundError{Reason: "No registered service(s) matching name", ErrorMessage: "invalid_service"}

}

func validateRules(filters []resources.Rule) error {
	for _, filter := range filters {

		if filter.Destination == "" {
			return &InvalidRuleError{Reason: "invalid destination", ErrorMessage: "invalid_destination"}
		}

		if filter.AbortProbability < 0.0 || filter.AbortProbability > 1.0 {
			return &InvalidRuleError{Reason: "invalid abort probability", ErrorMessage: "invalid_abort_probability"}
		}

		if filter.ReturnCode < 0 || filter.ReturnCode >= 600 {
			return &InvalidRuleError{Reason: "invalid return code", ErrorMessage: "invalid_return_code"}
		}

		if filter.DelayProbability < 0.0 || filter.DelayProbability > 1.0 {
			return &InvalidRuleError{Reason: "invalid probability", ErrorMessage: "invalid_delay_probability"}
		}

		if filter.Delay < 0 || filter.Delay > 600 {
			return &InvalidRuleError{Reason: "invalid delay", ErrorMessage: "invalid_delay"}
		}

		if (filter.DelayProbability != 0.0 && filter.Delay == 0.0) || (filter.DelayProbability == 0.0 && filter.Delay != 0.0) {
			return &InvalidRuleError{Reason: "invalid delay", ErrorMessage: "invalid_delay"}
		}

		// if filter.Header == "" {
		// 	filter.Header = "X-Filter-Header"
		// }

		if filter.Pattern == "" {
			filter.Pattern = "*"
		}

	}

	return nil
}

func (m *manager) updateProxyConfig(entry resources.TenantEntry) error {

	if err := m.db.Update(entry); err != nil {
		if ce, ok := err.(*database.DBError); ok {
			if ce.StatusCode == http.StatusConflict {
				newerEntry, err := m.db.Read(entry.ID)
				if err != nil {
					logrus.WithFields(logrus.Fields{
						"err": err,
						"id":  entry.ID,
					}).Error("Failed to retrieve latest document during conflict resolution")
					return err
				}

				newerEntry.ProxyConfig = entry.ProxyConfig

				if err = m.db.Update(entry); err != nil {
					logrus.WithFields(logrus.Fields{
						"err": err,
						"id":  entry.ID,
					}).Error("Failed to resolve document update conflict")
					return err
				}
				logrus.WithFields(logrus.Fields{
					"id": entry.ID,
				}).Debug("Succesfully resolved document update conflict")
				return nil
			}
			logrus.WithFields(logrus.Fields{
				"err": err,
				"id":  entry.ID,
			}).Error("Database error attempting to update proxy config")
			return err

		}
		logrus.WithFields(logrus.Fields{
			"err": err,
			"id":  entry.ID,
		}).Error("Failed attempting to update proxy config")
		return err
	}

	return nil
}
