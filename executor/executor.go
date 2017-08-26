package executor
 
import (
       "encoding/json"
       "errors"
       "fmt"
       "strconv"
       "strings"
 
       "github-beta.huawei.com/hipaas/common/crypto/aes"
       "github-beta.huawei.com/hipaas/common/storage/app"
       "github-beta.huawei.com/hipaas/common/storage/createstate"
       "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github.com/fsouza/go-dockerclient"
)

func ParaRunContainerOption(app *app.App, deployInfo *node.DeployInfo) (*createstate.RunContainerOption, error) {
       if app.Recovery == 0 {
              deployInfo.Recovery = strconv.FormatBool(false)
       } else {
              deployInfo.Recovery = strconv.FormatBool(true)
       }
       /************************create option**********************/
       envsMap := make(map[string]string)
       for _, env := range app.Envs {
              envsMap[env.K] = env.V
       }
 
       userDefinedEnvs, err := app.Envs.ToString()
       if err != nil {
              return nil, fmt.Errorf("serielize envs error: %s", err.Error())
       }
	   mountsString, err := app.Mount.ToString()
       if err != nil {
              return nil, fmt.Errorf("serielize mount error: %s", err.Error())
       }
       healthString, err := app.Health.ToString()
       if err != nil {
              return nil, fmt.Errorf("serielize health error: %s", err.Error())
       }
       archive := realstate.AppArchive{
              Mount:     mountsString,
              Cmd:       app.Cmd,
              Health:    healthString,
              Keeproute: app.Keeproute,
              Gateway:   deployInfo.Gateway,
              Mask:      deployInfo.Mask,
       }
       archiveJson, err := json.Marshal(archive)
       if nil != err {
              return nil, fmt.Errorf("marshal archive failed. Error: %v", err)
       }
 
       jsonport, err := app.Ports.ToString()
       if nil != err {
              return nil, fmt.Errorf("marhshal app.ports failed. Error: %v.", err)
       }
	   hipaasLabels := make(map[string]string)
       hipaasLabels[realstate.ContainerEnv[realstate.Envs]] = userDefinedEnvs
       hipaasLabels[realstate.ContainerEnv[realstate.Recovery]] = deployInfo.Recovery
       hipaasLabels[realstate.ContainerEnv[realstate.HostName]] = deployInfo.Hostname
       hipaasLabels[realstate.ContainerEnv[realstate.ContainerIP]] = deployInfo.ContainerIP
       hipaasLabels[realstate.ContainerEnv[realstate.Cpu]] = fmt.Sprintf("%d", app.CPU)
       hipaasLabels[realstate.ContainerEnv[realstate.Memory]] = fmt.Sprintf("%d", app.Memory)
       hipaasLabels[realstate.ContainerEnv[realstate.AppName]] = app.Name
       hipaasLabels[realstate.ContainerEnv[realstate.AppType]] = app.AppType
	   hipaasLabels[realstate.ContainerEnv[realstate.CtnrArchive]] = string(archiveJson)
       hipaasLabels[realstate.ContainerEnv[realstate.Ports]] = jsonport
 
       exposePorts := make(map[docker.Port]struct{})
       portBindings := make(map[docker.Port][]docker.PortBinding)
       valueStr := ""
       for _, port := range app.Ports {
              valueStr = fmt.Sprintf("%d", port.Port)
              exposePorts[docker.Port(valueStr+"/tcp")] = struct{}{}
              portBindings[docker.Port(valueStr+"/tcp")] = []docker.PortBinding{docker.PortBinding{}}
       }
 
       cpuShares := int64(app.CPU * 1024) // 1024 is the default weighting of CPU shares
       memoryLimit := int64(app.Memory * 1024 * 1024)
 
       createOption := docker.CreateContainerOptions{
              Name: deployInfo.Ctnrname,
			  Config: &docker.Config{
                     CPUShares:    cpuShares,
                     Memory:       memoryLimit,
                     ExposedPorts: exposePorts,
                     Image:        app.Image.DockerImageURL,
                     AttachStdin:  false,
                     AttachStdout: false,
                     AttachStderr: false,
                     Env:          buildEnvArray(envsMap),
                     Hostname:     app.GetSubHostname(deployInfo.Hostname),
                     Labels:       hipaasLabels,
              },
       }
 
       if app.Cmd != "" {
              createOption.Config.Entrypoint = strings.Split(app.Cmd, " ")
       }
 
       /************************auth option**********************/
       var auth docker.AuthConfiguration
       if app.Image.DockerLoginServer == "" || len(app.Image.DockerLoginServer) == 0 {
              auth = docker.AuthConfiguration{}
       } else {
              myaes, err := aes.New("")
			  if err != nil {
                     return nil, err
              }
              password, err := myaes.Decrypt(app.Image.DockerPassword)
              if err != nil {
                     return nil, errors.New("app.Image.DockerPassword decrypt error: "   err.Error())
              }
              auth = docker.AuthConfiguration{
                     Username:      app.Image.DockerUser,
                     Password:      password,
                     Email:         app.Image.DockerEmail,
                     ServerAddress: app.Image.DockerLoginServer,
              }
       }
       /************************mount option**********************/
       mount, oprate, err := parseMount(app.Mount, deployInfo.Ctnrname)
       if err != nil {
              return &createstate.RunContainerOption{}, err
       }
	   /************************start option**********************/
       extraHost := deployInfo.Hostname   " "   app.GetSubHostname(deployInfo.Hostname)   ":"   deployInfo.ContainerIP
       hostConfig := &docker.HostConfig{
              CPUShares:    cpuShares,
              Memory:       memoryLimit,
              PortBindings: portBindings,
              Privileged:   true,
              ExtraHosts:   []string{extraHost},
              DNSSearch:    []string{app.GetSubDomain(deployInfo.Hostname)},
              LogConfig: docker.LogConfig{
                     Type:   "json-file",
                     Config: map[string]string{"max-size": "40m", "max-file": "5"},
              },
       }
 
       if deployInfo.ContainerIP != "" {
              hostConfig.NetworkMode = "none"
       }
       if len(mount) != 0 || mount != nil {
              hostConfig.Binds = mount
       }
	   /************************image option**********************/
       repos, tag := parseRepositoryTag(app.Image.DockerImageURL)
       /************************run option**********************/
       return &createstate.RunContainerOption{
              MountOption:  oprate,
              CreateOption: createOption,
              ImageOption:  docker.PullImageOptions{Repository: repos, Tag: tag},
              AuthOption:   auth,
              StartOption:  *hostConfig,
              ActionOption: *deployInfo,
       }, nil
}
 
func buildEnvArray(envVars map[string]string) []string {
       size := len(envVars)
       if size == 0 {
              return []string{}
       }
 
       arr := make([]string, size)
       idx := 0
       for k, v := range envVars {
              if k == "" {
                     continue
              }
              arr[idx] = fmt.Sprintf("%s=%s", k, v)
              idx++
       }
 
       return arr
}

// 返回挂载点和处理过的mount结构体
func parseMount(mounts app.Mounts, ctnrname string) ([]string, app.Mounts, error) {
       var binds []string
       var bindMounts app.Mounts
       for _, mount := range mounts {
              if err := mount.IsValid(); err != nil {
                     return nil, nil, err
              }
              if mount.Type == app.MountTypePrivate {
                     mount.HPath = ParaPath(mount.HPath)   "/"   ctnrname
              }
              bind := mount.HPath   ":"   mount.CPath
              if mount.Privilege != "" {
                     bind  = ":"   mount.Privilege
              }
 
              binds = append(binds, bind)
              bindMounts = append(bindMounts, mount)
       }
       return binds, bindMounts, nil
}

func ParaPath(path string) string {
       if len(path) > 1 && string(path[len(path)-1]) == "/" {
              return path[:len(path)-1]
       }
       return path
}
 
func parseRepositoryTag(repos string) (string, string) {
       n := strings.LastIndex(repos, ":")
       if n < 0 {
              return repos, ""
       }
       if tag := repos[n 1:]; !strings.Contains(tag, "/") {
              return repos[:n], tag
       }
       return repos, ""
}
