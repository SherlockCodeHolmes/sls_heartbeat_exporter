package collector

import (
	"fmt"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	sls "github.com/alibabacloud-go/sls-20201230/v5/client"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"sync"
)

func CreateClient(accessKeyId *string, accessKeySecret *string, regionId string) *sls.Client {
	config := &openapi.Config{
		// 必填，您的 AccessKey ID
		AccessKeyId: accessKeyId,
		// 必填，您的 AccessKey Secret
		AccessKeySecret: accessKeySecret,
		// 默认为 cn-hangzhou
		RegionId: tea.String(regionId),
	}
	client, err := sls.NewClient(config)
	if err != nil {
		panic(err)
	}
	return client
}

type SlsExporter struct {
	auth                Auth
	slsMachineHeartbeat *prometheus.GaugeVec
	projectConf         map[string][]string
}

func NewSlsExporter(file string) *SlsExporter {
	slsMachineHeartbeat := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sls_machine_heartbeat",
			Help: "Heartbeat status of machines",
		},
		[]string{"machine", "region", "project", "machine_group"},
	)
	conf, err := InitExporterConf(file)
	if err != nil {
		panic(err)
	}
	return &SlsExporter{
		auth:                conf.Auth,
		slsMachineHeartbeat: slsMachineHeartbeat,
		projectConf:         conf.ProjectConf,
	}
}

func listMachineGroup(client *sls.Client, project string) ([]*string, error) {
	listMachineGroupRequest := &sls.ListMachineGroupRequest{}
	result, err := client.ListMachineGroup(tea.String(project), listMachineGroupRequest)
	if err != nil {
		return nil, err
	}
	return result.Body.Machinegroups, nil
}

func listMachine(client *sls.Client, project string, machineGroup *string) ([]string, error) {
	listMachineRequest := &sls.ListMachinesRequest{}
	result, err := client.ListMachines(&project, machineGroup, listMachineRequest)
	if err != nil {
		return nil, err
	}
	allMachines := make([]string, 0)
	for _, machine := range result.Body.Machines {
		allMachines = append(allMachines, *machine.Ip)
	}
	return allMachines, nil
}

func getMachineGroup(client *sls.Client, project string, machineGroup *string) ([]*string, error) {
	result, err := client.GetMachineGroup(&project, machineGroup)
	if err != nil {
		return nil, err
	}
	return result.Body.MachineList, nil

}

func (s *SlsExporter) processMachineGroup(client *sls.Client, regionId, project string, machineGroup *string) {
	region, err := extractPart(regionId)
	if err != nil {
		return // 或者处理错误
	}
	shouldMachines, err := getMachineGroup(client, project, machineGroup)
	if err != nil {
		return // 或者处理错误
	}
	actualMachines, err := listMachine(client, project, machineGroup)
	if err != nil {
		return // 或者处理错误
	}

	// 创建应有的机器集合
	shouldMachineSet := make(map[string]bool)
	for _, machine := range shouldMachines {
		shouldMachineSet[*machine] = true
	}

	// 更新实际存在的机器的指标
	for _, machine := range actualMachines {
		s.slsMachineHeartbeat.WithLabelValues(machine, region, project, *machineGroup).Set(1)
		delete(shouldMachineSet, machine)
	}

	// 更新应有而不存在的机器的指标
	for machine := range shouldMachineSet {
		s.slsMachineHeartbeat.WithLabelValues(machine, region, project, *machineGroup).Set(0)
	}
}

func extractPart(str string) (string, error) {
	parts := strings.Split(str, "-")
	if len(parts) < 2 {
		return "", fmt.Errorf("insufficient parts in the string: %s", str)
	}

	return parts[1], nil
}

func (s *SlsExporter) Describe(ch chan<- *prometheus.Desc) {
	s.slsMachineHeartbeat.Describe(ch)
}

func (s *SlsExporter) Collect(ch chan<- prometheus.Metric) {
	accessKey := s.auth.AccessKey
	secretKey := s.auth.SecretKey
	var wg sync.WaitGroup

	for region := range s.projectConf {
		client := CreateClient(&accessKey, &secretKey, region)
		for _, project := range s.projectConf[region] {
			machineGroups, err := listMachineGroup(client, project)
			if err != nil {
				continue // 或者处理错误
			}

			for _, machineGroup := range machineGroups {
				wg.Add(1)
				machineGroup := machineGroup
				go func() {
					defer wg.Done()
					s.processMachineGroup(client, region, project, machineGroup)
				}()
			}
		}
	}

	wg.Wait()
	s.slsMachineHeartbeat.Collect(ch)
}
