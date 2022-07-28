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

//*done TODO
//TODO: middleware to remove trailing slash
//TODO: expose DHCP interface
//TODO: get rid of global variable

//! todo TODO
//TODO: access mac address from DHCP interface
//TODO: access all network data from DHCP interface
//TODO: make newline behavior consistent
//TODO: metadata endpoint
//TODO: ipv6, ipv4 printing behavior

func v0HegelMetadataHandler(logger log.Logger, client hardware.Client, rg *gin.RouterGroup) {
	userdata := rg.Group("/user-data")
	userdata.GET("", userdataHandler(logger, client))

	metadata := rg.Group("/meta-data")
	metadata.GET("/disks", diskHandler(logger, client))
	metadata.GET("/disks/:index", diskIndexHandler(logger, client))

	metadata.GET("/ssh-public-keys", sshHandler(logger, client))
	metadata.GET("/ssh-public-keys/:index", sshIndexHandler(logger, client))

	metadata.GET("/hostname", hostnameHandler(logger, client))
	metadata.GET("/gateway", gatewayHandler(logger, client))

	metadata.GET("/:mac", macHandler(logger, client))
	metadata.GET("/:mac/ipv4", ipv4Handler(logger, client))
	metadata.GET("/:mac/ipv4/:index", ipv4IndexHandler(logger, client))
	metadata.GET("/:mac/ipv4/:index/ip", ipv4IPHandler(logger, client))
	metadata.GET("/:mac/ipv4/:index/netmask", ipv4NetmaskHandler(logger, client))
	metadata.GET("/:mac/ipv6", ipv6Handler(logger, client))
	metadata.GET("/:mac/ipv6/:index", ipv6IndexHandler(logger, client))
	metadata.GET("/:mac/ipv6/:index/ip", ipv6IPHandler(logger, client))
	metadata.GET("/:mac/ipv6/:index/netmask", ipv6NetmaskHandler(logger, client))
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

func userdataHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func diskHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func diskIndexHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func sshHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func sshIndexHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func hostnameHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func gatewayHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func macHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func ipv4Handler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func ipv4IndexHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func ipv4IPHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func ipv4NetmaskHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func ipv6Handler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func ipv6IndexHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func ipv6IPHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}

func ipv6NetmaskHandler(logger log.Logger, client hardware.Client) gin.HandlerFunc {
	fn := func(c *gin.Context) {
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
	return gin.HandlerFunc(fn)
}
