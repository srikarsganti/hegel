package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/packethost/pkg/log"
	"github.com/tinkerbell/hegel/hardware"
)

var logger log.Logger
var client hardware.Client

//TODO: middleware to remove trailing slash
//TODO: expose DHCP interface
//TODO: access mac address from DHCP interface
//TODO: access all network data from DHCP interface
//TODO: make newline behavior consistent
func v0HegelMetadataHandler(loggerHandler log.Logger, clientData hardware.Client, rg *gin.RouterGroup) {
	logger = loggerHandler
	client = clientData

	userdata := rg.Group("/user-data")
	userdata.GET("", userdataHandler)

	metadata := rg.Group("/meta-data")
	metadata.GET("/disks", diskHandler)
	metadata.GET("/disks/:index", diskIndexHandler)

	metadata.GET("/ssh-public-keys", sshHandler)
	metadata.GET("/ssh-public-keys/:index", sshIndexHandler)

	metadata.GET("/hostname", hostnameHandler)
	metadata.GET("/gateway", gatewayHandler)

	metadata.GET("/:mac", macHandler)
	metadata.GET("/:mac/ipv4", ipv4Handler)
	metadata.GET("/:mac/ipv4/:index", ipv4IndexHandler)
	metadata.GET("/:mac/ipv4/:index/ip", ipv4IPHandler)
	metadata.GET("/:mac/ipv4/:index/netmask", ipv4NetmaskHandler)
	metadata.GET("/:mac/ipv6", ipv6Handler)
	metadata.GET("/:mac/ipv6/:index", ipv6IndexHandler)
	metadata.GET("/:mac/ipv6/:index/ip", ipv6IPHandler)
	metadata.GET("/:mac/ipv6/:index/netmask", ipv6NetmaskHandler)
}

func getHardware(ctx context.Context, client hardware.Client, ip string) (hardware.K8sHardware, error) {
	hw, err := client.ByIP(ctx, ip)
	if err != nil {
		return hardware.K8sHardware{}, err
	}

	ehw, err := hw.Export()
	if err != nil {
		return hardware.K8sHardware{}, err
	}

	var reversed hardware.K8sHardware
	if err := json.Unmarshal(ehw, &reversed); err != nil {
		return hardware.K8sHardware{}, err
	}
	return reversed, nil
}

func userdataHandler(c *gin.Context) {
	hardware, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	data := hardware.Metadata.Userdata
	if data == nil {
		c.String(http.StatusOK, "")
	} else {
		c.String(http.StatusOK, *data)
	}
}

func diskHandler(c *gin.Context) {
	hardware, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	disk := hardware.Metadata.Instance.Disks
	for i := 0; i < len(disk); i++ {
		c.String(http.StatusOK, fmt.Sprintln(i))
	}
}

func diskIndexHandler(c *gin.Context) {
	hardware, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		logger.With("error", err).Info("disk interface index is not a valid number")
		c.JSON(http.StatusBadRequest, nil)
		return
	}
	disksArray := hardware.Metadata.Instance.Disks
	if index >= 0 && index < len(disksArray) {
		disk := hardware.Metadata.Instance.Disks[index]
		c.JSON(http.StatusOK, disk)
	} else {
		c.JSON(http.StatusBadRequest, nil)
		//? is this the best thing to return
	}
}

func sshHandler(c *gin.Context) {
	hardware, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	sshKeys := hardware.Metadata.Instance.SSHKeys
	for i := 0; i < len(sshKeys); i++ {
		c.String(http.StatusOK, fmt.Sprintln(i))
	}
}

func sshIndexHandler(c *gin.Context) {
	hardware, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		logger.With("error", err).Info("disk interface index is not a valid number")
		c.JSON(http.StatusBadRequest, nil)
		return
	}
	sshKeys := hardware.Metadata.Instance.SSHKeys
	if index >= 0 && index < len(sshKeys) {
		ssh := hardware.Metadata.Instance.SSHKeys[index]
		c.String(http.StatusOK, ssh)
	} else {
		c.String(http.StatusBadRequest, "")
		//? is this the best thing to return
	}
}

func hostnameHandler(c *gin.Context) {
	hardware, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	hostname := hardware.Metadata.Instance.Hostname
	c.String(http.StatusOK, hostname)
	//? additional security
}

func gatewayHandler(c *gin.Context) {
	hardware, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	gateway := hardware.Metadata.Instance.Network.Addresses[0].Gateway //! err check
	c.String(http.StatusOK, gateway)
	//? additional security?

}

func macHandler(c *gin.Context) {
	hardware, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	mac := c.Param("mac")
	if mac != hardware.Metadata.Instance.ID {
		c.String(http.StatusNoContent, "")
	} else {
		networkInfo := hardware.Metadata.Instance.Network.Addresses
		availableIP := map[string]bool{
			"ipv4": false,
			"ipv6": false,
		}
		for _, v := range networkInfo {
			if v.AddressFamily == 4 {
				availableIP["ipv4"] = true
			} else if v.AddressFamily == 6 {
				availableIP["ipv6"] = true
			}
		}
		var addressIsAvailable bool = false
		for key, value := range availableIP {
			if value {
				c.String(http.StatusOK, fmt.Sprintln(key))
				addressIsAvailable = true
			}
		}
		if !addressIsAvailable {
			c.String(http.StatusNoContent, "")
		}
	}
}

func ipv4Handler(c *gin.Context) {
	hardware, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	mac := c.Param("mac")
	if mac != hardware.Metadata.Instance.ID {
		c.String(http.StatusNoContent, "")
	} else {
		networkInfo := hardware.Metadata.Instance.Network.Addresses
		i := 0
		for _, v := range networkInfo {
			if v.AddressFamily == 4 {
				//* printing the indexes
				c.String(http.StatusOK, fmt.Sprintln(i))
				i++
			}
		}
		if i == 0 {
			c.String(http.StatusNoContent, "")
		}
	}
}

func ipv4IndexHandler(c *gin.Context) {
	hardwareStruct, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	mac := c.Param("mac")
	if mac != hardwareStruct.Metadata.Instance.ID {
		c.String(http.StatusNoContent, "")
	} else {
		networkInfo := hardwareStruct.Metadata.Instance.Network.Addresses
		var ipv4Networks []hardware.K8sHardwareMetadataInstanceNetworkAddress
		for _, v := range networkInfo {
			if v.AddressFamily == 4 {
				ipv4Networks = append(ipv4Networks, v)
			}
		}
		if len(ipv4Networks) == 0 {
			c.String(http.StatusNoContent, "")
		} else {
			c.String(http.StatusOK, "ip\nnetmask")
		}
	}
}

func ipv4IPHandler(c *gin.Context) {
	hardwareStruct, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	mac := c.Param("mac")
	if mac != hardwareStruct.Metadata.Instance.ID {
		c.String(http.StatusNoContent, "")
	} else {
		index, err := strconv.Atoi(c.Param("index"))
		if err != nil {
			logger.With("error", err).Info("ipv4 interface index is not a valid number")
			c.JSON(http.StatusBadRequest, nil)
			return
		}
		networkInfo := hardwareStruct.Metadata.Instance.Network.Addresses
		var ipv4Networks []hardware.K8sHardwareMetadataInstanceNetworkAddress
		for _, v := range networkInfo {
			if v.AddressFamily == 4 {
				ipv4Networks = append(ipv4Networks, v)
			}
		}
		if len(ipv4Networks) == 0 || index < 0 || index >= len(ipv4Networks) {
			c.String(http.StatusNoContent, "")
		} else {
			ip := ipv4Networks[index].Address
			c.String(http.StatusOK, ip)
		}
	}
}

func ipv4NetmaskHandler(c *gin.Context) {
	hardwareStruct, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	mac := c.Param("mac")
	if mac != hardwareStruct.Metadata.Instance.ID {
		c.String(http.StatusNoContent, "")
	} else {
		index, err := strconv.Atoi(c.Param("index"))
		if err != nil {
			logger.With("error", err).Info("ipv4 interface index is not a valid number")
			c.JSON(http.StatusBadRequest, nil)
			return
		}
		networkInfo := hardwareStruct.Metadata.Instance.Network.Addresses
		var ipv4Networks []hardware.K8sHardwareMetadataInstanceNetworkAddress
		for _, v := range networkInfo {
			if v.AddressFamily == 4 {
				ipv4Networks = append(ipv4Networks, v)
			}
		}
		if len(ipv4Networks) == 0 || index < 0 || index >= len(ipv4Networks) {
			c.String(http.StatusNoContent, "")
		} else {
			netmask := ipv4Networks[index].Netmask
			c.String(http.StatusOK, netmask)
		}
	}
}

func ipv6Handler(c *gin.Context) {
	hardware, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	mac := c.Param("mac")
	if mac != hardware.Metadata.Instance.ID {
		c.String(http.StatusNoContent, "")
	} else {
		networkInfo := hardware.Metadata.Instance.Network.Addresses
		i := 0
		for _, v := range networkInfo {
			if v.AddressFamily == 6 {
				//* printing the indexes
				c.String(http.StatusOK, fmt.Sprintln(i))
				i++
			}
		}
		if i == 0 {
			c.String(http.StatusNoContent, "")
		}
	}
}

func ipv6IndexHandler(c *gin.Context) {
	hardwareStruct, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	mac := c.Param("mac")
	if mac != hardwareStruct.Metadata.Instance.ID {
		c.String(http.StatusNoContent, "")
	} else {
		networkInfo := hardwareStruct.Metadata.Instance.Network.Addresses
		var ipv6Networks []hardware.K8sHardwareMetadataInstanceNetworkAddress
		for _, v := range networkInfo {
			if v.AddressFamily == 6 {
				ipv6Networks = append(ipv6Networks, v)
			}
		}
		if len(ipv6Networks) == 0 {
			c.String(http.StatusNoContent, "")
		} else {
			c.String(http.StatusOK, "ip\nnetmask")
		}
	}
}

func ipv6IPHandler(c *gin.Context) {
	hardwareStruct, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	mac := c.Param("mac")
	if mac != hardwareStruct.Metadata.Instance.ID {
		c.String(http.StatusNoContent, "")
	} else {
		index, err := strconv.Atoi(c.Param("index"))
		if err != nil {
			logger.With("error", err).Info("ipv6 interface index is not a valid number")
			c.JSON(http.StatusBadRequest, nil)
			return
		}
		networkInfo := hardwareStruct.Metadata.Instance.Network.Addresses
		var ipv6Networks []hardware.K8sHardwareMetadataInstanceNetworkAddress
		for _, v := range networkInfo {
			if v.AddressFamily == 6 {
				ipv6Networks = append(ipv6Networks, v)
			}
		}
		if len(ipv6Networks) == 0 || index < 0 || index >= len(ipv6Networks) {
			c.String(http.StatusNoContent, "")
		} else {
			ip := ipv6Networks[index].Address
			c.String(http.StatusOK, ip)
		}
	}
}

func ipv6NetmaskHandler(c *gin.Context) {
	hardwareStruct, err := getHardware(c, client, c.ClientIP())
	if err != nil {
		logger.With("error", err).Info("failed to get hardware in v0 metadata handler")
		c.JSON(http.StatusNotFound, nil)
		return
	}
	mac := c.Param("mac")
	if mac != hardwareStruct.Metadata.Instance.ID {
		c.String(http.StatusNoContent, "")
	} else {
		index, err := strconv.Atoi(c.Param("index"))
		if err != nil {
			logger.With("error", err).Info("ipv6 interface index is not a valid number")
			c.JSON(http.StatusBadRequest, nil)
			return
		}
		networkInfo := hardwareStruct.Metadata.Instance.Network.Addresses
		var ipv6Networks []hardware.K8sHardwareMetadataInstanceNetworkAddress
		for _, v := range networkInfo {
			if v.AddressFamily == 6 {
				ipv6Networks = append(ipv6Networks, v)
			}
		}
		if len(ipv6Networks) == 0 || index < 0 || index >= len(ipv6Networks) {
			c.String(http.StatusNoContent, "")
		} else {
			netmask := ipv6Networks[index].Netmask
			c.String(http.StatusOK, netmask)
		}
	}
}
