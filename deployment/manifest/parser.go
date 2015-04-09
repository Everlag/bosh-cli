package manifest

import (
	"github.com/cloudfoundry-incubator/candiedyaml"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshsys "github.com/cloudfoundry/bosh-agent/system"

	biproperty "github.com/cloudfoundry/bosh-init/common/property"
)

type Parser interface {
	Parse(path string) (Manifest, error)
}

type parser struct {
	fs     boshsys.FileSystem
	logger boshlog.Logger
	logTag string
}

type manifest struct {
	Name          string
	Update        UpdateSpec
	Networks      []network
	ResourcePools []resourcePool `yaml:"resource_pools"`
	DiskPools     []diskPool     `yaml:"disk_pools"`
	Jobs          []job
	Properties    map[interface{}]interface{}
}

type UpdateSpec struct {
	UpdateWatchTime *string `yaml:"update_watch_time"`
}

type network struct {
	Name            string                      `yaml:"name"`
	Type            string                      `yaml:"type"`
	CloudProperties map[interface{}]interface{} `yaml:"cloud_properties"`
	IP              string                      `yaml:"ip"`
	Netmask         string                      `yaml:"netmask"`
	Gateway         string                      `yaml:"gateway"`
	DNS             []string                    `yaml:"dns"`
}

type resourcePool struct {
	Name            string                      `yaml:"name"`
	Network         string                      `yaml:"network"`
	CloudProperties map[interface{}]interface{} `yaml:"cloud_properties"`
	Env             map[interface{}]interface{} `yaml:"env"`
}

type diskPool struct {
	Name            string                      `yaml:"name"`
	DiskSize        int                         `yaml:"disk_size"`
	CloudProperties map[interface{}]interface{} `yaml:"cloud_properties"`
}

type job struct {
	Name               string
	Instances          int
	Lifecycle          string
	Templates          []releaseJobRef
	Networks           []jobNetwork
	PersistentDisk     int    `yaml:"persistent_disk"`
	PersistentDiskPool string `yaml:"persistent_disk_pool"`
	Properties         map[interface{}]interface{}
}

type releaseJobRef struct {
	Name    string
	Release string
}

type jobNetwork struct {
	Name      string
	Default   []string
	StaticIPs []string `yaml:"static_ips"`
}

var boshDeploymentDefaults = Manifest{
	Update: Update{
		UpdateWatchTime: WatchTime{
			Start: 0,
			End:   300000,
		},
	},
}

func NewParser(fs boshsys.FileSystem, logger boshlog.Logger) Parser {
	return &parser{
		fs:     fs,
		logger: logger,
		logTag: "deploymentParser",
	}
}

func (p *parser) Parse(path string) (Manifest, error) {
	contents, err := p.fs.ReadFile(path)
	if err != nil {
		return Manifest{}, bosherr.WrapErrorf(err, "Reading file %s", path)
	}

	comboManifest := manifest{}
	err = candiedyaml.Unmarshal(contents, &comboManifest)
	if err != nil {
		return Manifest{}, bosherr.WrapError(err, "Unmarshalling BOSH deployment manifest")
	}
	p.logger.Debug(p.logTag, "Parsed BOSH deployment manifest: %#v", comboManifest)

	deploymentManifest, err := p.parseDeploymentManifest(comboManifest)
	if err != nil {
		return Manifest{}, bosherr.WrapError(err, "Unmarshalling BOSH deployment manifest")
	}

	return deploymentManifest, nil
}

func (p *parser) parseDeploymentManifest(depManifest manifest) (Manifest, error) {
	deployment := boshDeploymentDefaults
	deployment.Name = depManifest.Name

	networks, err := p.parseNetworkManifests(depManifest.Networks)
	if err != nil {
		return Manifest{}, bosherr.WrapErrorf(err, "Parsing networks: %#v", depManifest.Networks)
	}
	deployment.Networks = networks

	resourcePools, err := p.parseResourcePoolManifests(depManifest.ResourcePools)
	if err != nil {
		return Manifest{}, bosherr.WrapErrorf(err, "Parsing resource_pools: %#v", depManifest.ResourcePools)
	}
	deployment.ResourcePools = resourcePools

	diskPools, err := p.parseDiskPoolManifests(depManifest.DiskPools)
	if err != nil {
		return Manifest{}, bosherr.WrapErrorf(err, "Parsing disk_pools: %#v", depManifest.DiskPools)
	}
	deployment.DiskPools = diskPools

	jobs, err := p.parseJobManifests(depManifest.Jobs)
	if err != nil {
		return Manifest{}, bosherr.WrapErrorf(err, "Parsing jobs: %#v", depManifest.Jobs)
	}
	deployment.Jobs = jobs

	properties, err := biproperty.BuildMap(depManifest.Properties)
	if err != nil {
		return Manifest{}, bosherr.WrapErrorf(err, "Parsing global manifest properties: %#v", depManifest.Properties)
	}
	deployment.Properties = properties

	if depManifest.Update.UpdateWatchTime != nil {
		updateWatchTime, err := NewWatchTime(*depManifest.Update.UpdateWatchTime)
		if err != nil {
			return Manifest{}, bosherr.WrapError(err, "Parsing update watch time")
		}

		deployment.Update = Update{
			UpdateWatchTime: updateWatchTime,
		}
	}

	return deployment, nil
}

func (p *parser) parseJobManifests(rawJobs []job) ([]Job, error) {
	jobs := make([]Job, len(rawJobs), len(rawJobs))
	for i, rawJob := range rawJobs {
		job := Job{
			Name:               rawJob.Name,
			Instances:          rawJob.Instances,
			Lifecycle:          JobLifecycle(rawJob.Lifecycle),
			PersistentDisk:     rawJob.PersistentDisk,
			PersistentDiskPool: rawJob.PersistentDiskPool,
		}

		if rawJob.Templates != nil {
			releaseJobRefs := make([]ReleaseJobRef, len(rawJob.Templates), len(rawJob.Templates))
			for i, rawJobRef := range rawJob.Templates {
				releaseJobRefs[i] = ReleaseJobRef{
					Name:    rawJobRef.Name,
					Release: rawJobRef.Release,
				}
			}
			job.Templates = releaseJobRefs
		}

		if rawJob.Networks != nil {
			jobNetworks := make([]JobNetwork, len(rawJob.Networks), len(rawJob.Networks))
			for i, rawJobNetwork := range rawJob.Networks {
				jobNetwork := JobNetwork{
					Name:      rawJobNetwork.Name,
					StaticIPs: rawJobNetwork.StaticIPs,
				}

				if rawJobNetwork.Default != nil {
					networkDefaults := make([]NetworkDefault, len(rawJobNetwork.Default), len(rawJobNetwork.Default))
					for i, rawDefault := range rawJobNetwork.Default {
						networkDefaults[i] = NetworkDefault(rawDefault)
					}
					jobNetwork.Default = networkDefaults
				}

				jobNetworks[i] = jobNetwork
			}
			job.Networks = jobNetworks
		}

		if rawJob.Properties != nil {
			properties, err := biproperty.BuildMap(rawJob.Properties)
			if err != nil {
				return jobs, bosherr.WrapErrorf(err, "Parsing job '%s' properties: %#v", rawJob.Name, rawJob.Properties)
			}
			job.Properties = properties
		}

		jobs[i] = job
	}

	return jobs, nil
}

func (p *parser) parseNetworkManifests(rawNetworks []network) ([]Network, error) {
	networks := make([]Network, len(rawNetworks), len(rawNetworks))
	for i, rawNetwork := range rawNetworks {
		network := Network{
			Name:    rawNetwork.Name,
			Type:    NetworkType(rawNetwork.Type),
			IP:      rawNetwork.IP,
			Netmask: rawNetwork.Netmask,
			Gateway: rawNetwork.Gateway,
			DNS:     rawNetwork.DNS,
		}

		cloudProperties, err := biproperty.BuildMap(rawNetwork.CloudProperties)
		if err != nil {
			return networks, bosherr.WrapErrorf(err, "Parsing network '%s' cloud_properties: %#v", rawNetwork.Name, rawNetwork.CloudProperties)
		}
		network.CloudProperties = cloudProperties

		networks[i] = network
	}

	return networks, nil
}

func (p *parser) parseResourcePoolManifests(rawResourcePools []resourcePool) ([]ResourcePool, error) {
	resourcePools := make([]ResourcePool, len(rawResourcePools), len(rawResourcePools))
	for i, rawResourcePool := range rawResourcePools {
		resourcePool := ResourcePool{
			Name:    rawResourcePool.Name,
			Network: rawResourcePool.Network,
		}

		cloudProperties, err := biproperty.BuildMap(rawResourcePool.CloudProperties)
		if err != nil {
			return resourcePools, bosherr.WrapErrorf(err, "Parsing resource_pool '%s' cloud_properties: %#v", rawResourcePool.Name, rawResourcePool.CloudProperties)
		}
		resourcePool.CloudProperties = cloudProperties

		env, err := biproperty.BuildMap(rawResourcePool.Env)
		if err != nil {
			return resourcePools, bosherr.WrapErrorf(err, "Parsing resource_pool '%s' env: %#v", rawResourcePool.Name, rawResourcePool.Env)
		}
		resourcePool.Env = env

		resourcePools[i] = resourcePool
	}

	return resourcePools, nil
}

func (p *parser) parseDiskPoolManifests(rawDiskPools []diskPool) ([]DiskPool, error) {
	diskPools := make([]DiskPool, len(rawDiskPools), len(rawDiskPools))
	for i, rawDiskPool := range rawDiskPools {
		diskPool := DiskPool{
			Name:     rawDiskPool.Name,
			DiskSize: rawDiskPool.DiskSize,
		}

		cloudProperties, err := biproperty.BuildMap(rawDiskPool.CloudProperties)
		if err != nil {
			return diskPools, bosherr.WrapErrorf(err, "Parsing disk_pool '%s' cloud_properties: %#v", rawDiskPool.Name, rawDiskPool.CloudProperties)
		}
		diskPool.CloudProperties = cloudProperties

		diskPools[i] = diskPool
	}

	return diskPools, nil
}
