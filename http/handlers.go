package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/itchyny/gojq"
	"github.com/packethost/pkg/log"
	"github.com/pkg/errors"
	"github.com/tinkerbell/hegel/build"
	"github.com/tinkerbell/hegel/datamodel"
	"github.com/tinkerbell/hegel/grpc"
	"github.com/tinkerbell/hegel/hardware"
	"github.com/tinkerbell/hegel/metrics"
)

// ec2Filters defines the query pattern and filters for the EC2 endpoint
// for queries that are to return another list of metadata items, the filter is a static list of the metadata items ("directory-listing filter")
// for /meta-data, the `spot` metadata item will only show up when the instance is a spot instance (denoted by if the `spot` field inside hardware is nonnull)
// NOTE: make sure when adding a new metadata item in a "subdirectory", to also add it to the directory-listing filter.
var ec2Filters = map[string]string{
	"":                                    `"meta-data", "user-data"`, // base path
	"/user-data":                          ".metadata.userdata",
	"/meta-data":                          `["instance-id", "hostname", "local-hostname", "iqn", "plan", "facility", "tags", "operating-system", "public-keys", "public-ipv4", "public-ipv6", "local-ipv4"] + (if .metadata.instance.spot != null then ["spot"] else [] end) | sort | .[]`,
	"/meta-data/instance-id":              ".metadata.instance.id",
	"/meta-data/hostname":                 ".metadata.instance.hostname",
	"/meta-data/local-hostname":           ".metadata.instance.hostname",
	"/meta-data/iqn":                      ".metadata.instance.iqn",
	"/meta-data/plan":                     ".metadata.instance.plan",
	"/meta-data/facility":                 ".metadata.instance.facility",
	"/meta-data/tags":                     ".metadata.instance.tags[]?",
	"/meta-data/operating-system":         `["slug", "distro", "version", "license_activation", "image_tag"] | sort | .[]`,
	"/meta-data/operating-system/slug":    ".metadata.instance.operating_system.slug",
	"/meta-data/operating-system/distro":  ".metadata.instance.operating_system.distro",
	"/meta-data/operating-system/version": ".metadata.instance.operating_system.version",
	"/meta-data/operating-system/license_activation":       `"state"`,
	"/meta-data/operating-system/license_activation/state": ".metadata.instance.operating_system.license_activation.state",
	"/meta-data/operating-system/image_tag":                ".metadata.instance.operating_system.image_tag",
	"/meta-data/public-keys":                               ".metadata.instance.ssh_keys[]?",
	"/meta-data/spot":                                      `"termination-time"`,
	"/meta-data/spot/termination-time":                     ".metadata.instance.spot.termination_time",
	"/meta-data/public-ipv4":                               ".metadata.instance.network.addresses[]? | select(.address_family == 4 and .public == true) | .address",
	"/meta-data/public-ipv6":                               ".metadata.instance.network.addresses[]? | select(.address_family == 6 and .public == true) | .address",
	"/meta-data/local-ipv4":                                ".metadata.instance.network.addresses[]? | select(.address_family == 4 and .public == false) | .address",
}

var hegelFilters = map[string]string{
	"":                           `"metadata", "userdata"`, // base path
	"/user-data":                 ".metadata.userdata",
	"/meta-data":                 `["hostname", "disks", "public-ipv4", "public-ipv6", "local-ipv4", "gateway", "netmask"] | sort | .[]`,
	"/meta-data/hostname":        ".metadata.instance.hostname",
	"/meta-data/disks":           ".metadata.disks | length",
	"/meta-data/public-ipv4":     ".metadata.instance.network.addresses[]? | select(.address_family == 4 and .public == true) | .address",
	"/meta-data/public-ipv6":     ".metadata.instance.network.addresses[]? | select(.address_family == 6 and .public == true) | .address",
	"/meta-data/local-ipv4":      ".metadata.instance.network.addresses[]? | select(.address_family == 4 and .public == false) | .address",
	"/meta-data/gateway":         ".metadata.instance.network.addresses[]? | .gateway",
	"/meta-data/netmask":         ".metadata.instance.network.addresses[]? | .netmask",
	"/meta-data/ssh-public-keys": ".metadata.instance.ssh_keys | length",
}

func VersionHandler(logger log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := struct {
			// Use git_rev to match the health endpoint reporting.
			Revision string `json:"git_rev"`
		}{
			Revision: build.GetGitRevision(),
		}

		encoder := json.NewEncoder(w)

		if err := encoder.Encode(payload); err != nil {
			logger.Error(err, "marshalling version")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}

// HealthChecker provide health checking behavior for services.
type HealthChecker interface {
	IsHealthy(context.Context) bool
}

// HealthCheckHandler provides an http handler that exposes health check information to consumers.
// The data is exposed as a json payload containing git_rev, uptim, goroutines and hardware_client_status.
func HealthCheckHandler(logger log.Logger, client HealthChecker, start time.Time) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIsHealthy := client.IsHealthy(r.Context())

		res := struct {
			GitRev                  string  `json:"git_rev"`
			Uptime                  float64 `json:"uptime"`
			Goroutines              int     `json:"goroutines"`
			HardwareClientAvailable bool    `json:"hardware_client_status"`
		}{
			GitRev:                  build.GetGitRevision(),
			Uptime:                  time.Since(start).Seconds(),
			Goroutines:              runtime.NumGoroutine(),
			HardwareClientAvailable: clientIsHealthy,
		}

		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)

		if err := encoder.Encode(&res); err != nil {
			logger.Error(err, "Failed to write for healthChecker")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !clientIsHealthy {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}

// GetMetadataHandler provides an http handler that retrieves metadata using client filtering it
// using filter. filter should be a jq compatible processing string. Data is only filtered when
// using the TinkServer data model.
func GetMetadataHandler(logger log.Logger, client hardware.Client, filter string, model datamodel.DataModel) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		logger.Debug("retrieving metadata")
		userIP := getIPFromRequest(r)
		if userIP == "" {
			return
		}

		metrics.MetadataRequests.Inc()
		l := logger.With("userIP", userIP)
		l.Info("got ip from request")
		hw, err := client.ByIP(r.Context(), userIP)
		if err != nil {
			metrics.Errors.WithLabelValues("metadata", "lookup").Inc()
			l.With("error", err).Info("failed to get hardware by ip")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		hardware, err := hw.Export()
		if err != nil {
			l.With("error", err).Info("failed to export hardware")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if model == datamodel.TinkServer || model == datamodel.Kubernetes {
			hardware, err = filterMetadata(hardware, filter)
			if err != nil {
				l.With("error", err).Info("failed to filter metadata")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(hardware)
		if err != nil {
			l.With("error", err).Info("failed to write response")
		}
	})
}

func EC2MetadataHandler(logger log.Logger, client hardware.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		logger.Debug("calling EC2MetadataHandler")
		userIP := getIPFromRequest(r)
		if userIP == "" {
			logger.Info("Could not retrieve IP address")
			return
		}

		metrics.MetadataRequests.Inc()
		logger := logger.With("userIP", userIP)
		logger.Info("Retrieved IP peer IP")

		hw, err := client.ByIP(r.Context(), userIP)
		if err != nil {
			metrics.Errors.WithLabelValues("metadata", "lookup").Inc()
			logger.With("error", err).Info("failed to get hardware by ip")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		ehw, err := hw.Export()
		if err != nil {
			logger.With("error", err).Info("failed to export hardware")
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte("404 not found"))
			if err != nil {
				logger.With("error", err).Info("failed to write response")
			}
			return
		}

		logger.With("exported", string(ehw)).Debug("Exported hardware")

		filter, err := processEC2Query(r.URL.Path)
		if err != nil {
			logger.With("error", err).Info("failed to process ec2 query")
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte("404 not found"))
			if err != nil {
				logger.With("error", err).Info("failed to write response")
			}
			return
		}

		resp, err := filterMetadata(ehw, filter)
		if err != nil {
			logger.With("error", err).Info("failed to filter metadata")
		}

		_, err = w.Write(resp)
		if err != nil {
			logger.With("error", err).Info("failed to write response")
		}
	})
}

func HegelMetadataHandler(logger log.Logger, client hardware.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		logger.Debug("calling HegelMetadataHandler")
		userIP := getIPFromRequest(r)
		if userIP == "" {
			logger.Info("Could not retrieve IP address")
			return
		}

		metrics.MetadataRequests.Inc()
		logger := logger.With("userIP", userIP)
		logger.Info("Retrieved IP peer IP")

		hw, err := client.ByIP(r.Context(), userIP)
		if err != nil {
			metrics.Errors.WithLabelValues("metadata", "lookup").Inc()
			logger.With("error", err).Info("failed to get hardware by ip")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		ehw, err := hw.Export()
		if err != nil {
			logger.With("error", err).Info("failed to export hardware")
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte("404 not found"))
			if err != nil {
				logger.With("error", err).Info("failed to write response")
			}
			return
		}

		logger.With("exported", string(ehw)).Debug("Exported hardware")

		// fmt.Printf("request header: %T\n", r.Header["Accept"])
		filter, err := processHegelQuery(r.URL.Path, returnJSONObject(r.Header["Accept"]))
		if err != nil {
			logger.With("error", err).Info("failed to process hegel query")
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte("404 not found"))
			if err != nil {
				logger.With("error", err).Info("failed to write response")
			}
			return
		}

		resp, err := filterHegelMetadata(ehw, filter, r.URL.Path)
		if err != nil {
			logger.With("error", err).Info("failed to filter metadata")
		}

		if returnJSONObject(r.Header["Accept"]) {
			w.Header().Set("Content-Type", "application/json")
		}
		_, err = w.Write(resp)
		if err != nil {
			logger.With("error", err).Info("failed to write response")
		}
	})
}

func filterMetadata(hw []byte, filter string) ([]byte, error) {
	var result bytes.Buffer
	query, err := gojq.Parse(filter)
	if err != nil {
		return nil, err
	}
	input := make(map[string]interface{})
	err = json.Unmarshal(hw, &input)
	if err != nil {
		return nil, err
	}
	iter := query.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if v == nil {
			continue
		}

		switch vv := v.(type) {
		case error:
			return nil, errors.Wrap(vv, "error while filtering with gojq")
		case string:
			result.WriteString(vv)
		default:
			marshalled, err := json.Marshal(vv)
			if err != nil {
				return nil, errors.Wrap(err, "error marshalling jq result")
			}
			result.Write(marshalled)
		}
		result.WriteRune('\n')
	}

	return bytes.TrimSuffix(result.Bytes(), []byte("\n")), nil
}

func filterHegelMetadata(hw []byte, filter string, url string) ([]byte, error) {
	var result bytes.Buffer
	query, err := gojq.Parse(filter)
	if err != nil {
		return nil, err
	}
	input := make(map[string]interface{})
	err = json.Unmarshal(hw, &input)
	if err != nil {
		return nil, err
	}
	iter := query.Run(input)

	floatSet := make(map[float64]bool)
	networkObjectCounter := 0

	for {
		v, ok := iter.Next()
		if !ok {
			break
		}

		if v == nil {
			continue
		}

		switch vv := v.(type) {
		case error:
			return nil, errors.Wrap(vv, "error while filtering with gojq")
		case string:
			if isMACAddress(url) {
				switch routeLength(url) {
				case 5:
					networkIndex := getNetworkIndex(url)
					if networkIndex == networkObjectCounter {
						result.WriteString(vv)
						result.WriteRune('\n')
					}
					networkObjectCounter++
				default:
					result.WriteString(vv)
					result.WriteRune('\n')
				}
			} else {
				result.WriteString(vv)
				result.WriteRune('\n')
			}
		case int:
			if strings.Contains(url, "disks") {
				for i := 0; i < vv; i++ {
					marshalled, err := json.Marshal(i)
					if err != nil {
						return nil, errors.Wrap(err, "error marshalling jq result")
					}
					result.Write(marshalled)
					result.WriteRune('\n')
				}
			} else if strings.Contains(url, "ssh-public-keys") {
				for i := 0; i < vv; i++ {
					marshalled, err := json.Marshal(i)
					if err != nil {
						return nil, errors.Wrap(err, "error marshalling jq result")
					}
					result.Write(marshalled)
					result.WriteRune('\n')
				}
			}
		case float64:
			if isMACAddress(url) {
				switch routeLength(url) {
				case 2:
					if !floatSet[vv] {
						//* preventing from writing duplicate family addresses
						result.WriteString("ipv" + fmt.Sprint(vv))
						result.WriteRune('\n')
					}
					floatSet[vv] = true
				}
			}
		case map[string]interface{}:
			if isMACAddress(url) {
				switch routeLength(url) {
				case 3:
					marshalled, err := json.Marshal(networkObjectCounter)
					networkObjectCounter++
					if err != nil {
						return nil, errors.Wrap(err, "error marshalling jq result")
					}
					result.Write(marshalled)
					result.WriteRune('\n')
				}
			} else {
				marshalled, err := json.Marshal(vv)
				if err != nil {
					return nil, errors.Wrap(err, "error marshalling jq result")
				}
				result.Write(marshalled)
				result.WriteRune('\n')
			}
		default:
			marshalled, err := json.Marshal(vv)
			if err != nil {
				return nil, errors.Wrap(err, "error marshalling jq result")
			}
			result.Write(marshalled)
			result.WriteRune('\n')
		}
	}

	return bytes.TrimSuffix(result.Bytes(), []byte("\n")), nil
}

// processEC2Query returns either a specific filter (used to parse hardware data for the value of a specific field),
// or a comma-separated list of metadata items (to be printed).
func processEC2Query(url string) (string, error) {
	query := strings.TrimRight(strings.TrimPrefix(url, "/2009-04-04"), "/") // remove base pattern and trailing slash

	filter, ok := ec2Filters[query]
	if !ok {
		return "", errors.Errorf("invalid metadata item: %v", query)
	}

	return filter, nil
}

func processHegelQuery(url string, jsonRequest bool) (string, error) {
	query := strings.TrimRight(strings.TrimPrefix(url, "/v0"), "/") // remove base pattern and trailing slash

	if query == "/meta-data" && jsonRequest {
		filter := ".metadata.instance"
		return filter, nil
	} else if strings.Contains(query, "ssh-public-keys/") {
		index := strings.SplitAfter(query, "ssh-public-keys/")[1]
		filter := ".metadata.instance.ssh_keys[" + index + "]"
		return filter, nil
	} else if strings.Contains(query, "disks/") {
		index := strings.SplitAfter(query, "disks/")[1]
		filter := ".metadata.disks[" + index + "]"
		return filter, nil
	} else if isMACAddress(query) { //! improve method of verifying that we have a mac address in the query
		//* indicates that we have a mac address in the query
		split_query := strings.Split(query, "/")[1:] //* gets rid of empty character at the beginning
		mac := split_query[1]
		switch len(split_query) {
		case 2:
			filter := "select(.metadata.instance.id == \"" + mac + "\") |"
			filter += ".metadata.instance.network.addresses[]? | .address_family"
			return filter, nil
		case 3:
			address_family := strings.TrimPrefix(split_query[2], "ipv")
			filter := "select(.metadata.instance.id == \"" + mac + "\") |"
			filter += ".metadata.instance.network.addresses[]? | select(.address_family == " + address_family + ")"
			return filter, nil
		case 4:
			filter := `["ip", "netmask"] | .[]`
			return filter, nil
		case 5:
			address_family := strings.TrimPrefix(split_query[2], "ipv")
			filter := "select(.metadata.instance.id == \"" + mac + "\") |"
			filter += ".metadata.instance.network.addresses[]? | select(.address_family == " + address_family + ") |"
			if split_query[4] == "ip" {
				filter += ".address"
			} else if split_query[4] == "netmask" {
				filter += ".netmask"
			} else {
				return "", errors.Errorf("invalid endpoint: %v", query)
			}
			return filter, nil
		}
		filter := ".metadata.instance.network.addresses[]? | select(.address_family == 4 and .public == true) | .address"
		return filter, nil
	}

	filter, ok := hegelFilters[query]
	if !ok {
		return "", errors.Errorf("invalid metadata item: %v", query)
	}

	return filter, nil
}

func routeLength(url string) int {
	query := strings.TrimRight(strings.TrimPrefix(url, "/v0/"), "/") // remove base pattern and trailing slash
	split_query := strings.Split(query, "/")
	return len(split_query)
}

func isMACAddress(url string) bool {
	return strings.Count(url, ":") == 5
}

func getNetworkIndex(url string) int {
	query := strings.TrimRight(strings.TrimPrefix(url, "/v0/"), "/") // remove base pattern and trailing slash
	split_query := strings.Split(query, "/")
	val, _ := strconv.Atoi(split_query[3])
	return val
}

func returnJSONObject(httpHeaderAcceptType []string) bool {
	for _, accept := range httpHeaderAcceptType {
		if accept == "application/json" {
			return true
		}
	}
	return false
}

func getIPFromRequest(r *http.Request) string {
	addr := r.RemoteAddr
	if strings.ContainsRune(addr, ':') {
		addr, _, _ = net.SplitHostPort(addr)
	}
	return addr
}

func writeJSONError(w http.ResponseWriter, code int, err error) error {
	return writeJSONResponse(w, code, map[string]interface{}{
		"error": map[string]interface{}{
			"error":   err.Error(),
			"comment": "", // Maintained for backward compatibility.
		},
	})
}

func writeJSONResponse(w http.ResponseWriter, code int, payload interface{}) error {
	w.WriteHeader(code)
	return json.NewEncoder(w).Encode(payload)
}

func SubscriptionsHandler(server *grpc.Server, logger log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/subscriptions/")

		if id == "" {
			responseErr := fmt.Errorf("missing subscription id in path: %v", r.URL.Path)
			if err := writeJSONError(w, http.StatusNotFound, responseErr); err != nil {
				logger.Error(err)
			}
			return
		}

		subscription, err := server.Subscription(id)
		if err != nil {
			if err := writeJSONError(w, http.StatusNotFound, err); err != nil {
				logger.Error(err)
			}
			return
		}

		if err := writeJSONResponse(w, http.StatusOK, subscription); err != nil {
			logger.Error(err)
		}
	})
}
