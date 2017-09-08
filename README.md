# HiPaaS Scheduler
## Getting Started
### External Dependencies
 
- Go should be installed and in the PATH
- GOPATH should be set as described in http://golang.org/doc/code.html
- [godep](https://github.com/tools/godep) installed and in the PATH
- [MySQL](https://www.mysql.com/) should be installed, It would be best to install with source code
 
### Development Setup
```bash
mkdir -p $GOPATH/src/github-beta.huawei.com/hipaas
cd $GOPATH/src/github-beta.huawei.com/hipaas/
git clone http://github-beta.huawei.com/hipaas/scheduler.git
cd $GOPATH/src/github-beta.huawei.com/hipaas/scheduler
godep go build
```
### Running Start
```bash
cp cfg.example.json cfg.json
nohup ./scheduler -v=3 -alsologtostderr=true -log_dir= " ./log"&
```
 
## Usage Guides
This is HiPaaS control brain, mainly has the following functions
 
- 1. Open a RPC port to Receive the heartbeat informations of all th Agent and Collect loads and container lists of all compute nodes into memory, which called the real state.
- 2. It has a http port for debugging and exposing memory information.
- 3. Connect Mysql to get the target state of the current app regularly which is called state desired
- 4. Comparing the real state and the desired state. If container hang up, or expansion, it can compare the differences, and then create a new container or destroy the excess container.
- 5. Analyzed the container list Agent reported, organize the routing informations and write them into redis
- 6. Post the configuration of the scribe connection address into the container by writing environment variables, and App in container can push log to the scribe server.
 
## Q&A
**Q: If Scheduler was suspended, what should I do?**
 
**A:** Scheduler can be cold backup. Agent has two Scheduler addresses.，Usually the system starts a Scheduler. If the machine which Scheduler is located in was suspended, don't worry, just launch another, and The agent will automatically reconnect.
 
**Q: If the redis which stored the routing information was suspended, what should I do?**
 
**A:** After getting the route from the redis each time, Router will be cached to the local memory. It's not serious problem that Redis was suspended, just can't know the back end container changed。
 
## APP
INSERT into app(id,name,creator,team_id,team_name,region,image,mount,recovery,vmtype,memory,cpu,health,instance,status) values(1, 'name-test', 1, 1, 'team-test', '', '9.91.17.17:5000/docker-registry-ui:latest','', true,'was', 512, 2, 0, 0, 0);
 
INSERT into hostname(id, hostname, subdomain, log, status, app_id, app_name) values(1, 'test-hostname1', 'huawei.com', '', 0, 1, 'name-test');
 
INSERT into hostname(id, hostname, subdomain, log, status, app_id, app_name) values(2, 'test-hostname2', 'huawei.com', '', 0, 1, 'name-test');
 
INSERT into image(id, app_id, docker_image_url, docker_login_server, docker_user, docker_password, docker_email) values(1, 1, "9.91.17.17:5000/docker-registry-ui:latest", "", "", "", "")
 
## Resource Usage Monitoring in HiPaaS
- You can use "nohup" command to get debugging information, and also write log into file nohup.out. when you want to see the latest log, try the command "tail -f nohup.out".
 
 
## Configuration
Alternatively, HiPaaS can read its configuration from a JSON file in the following format:
```json
{
        "debug": true,
        "interval": 5,
        "dockerPort": 5555,
        "AgentRpcPort": 1990,
        "domain": "apps.io",
        "localIp": "127.0.0.1",
        "businessNIC": "eth0",
        "storage": "redis",
        "configCheckDuration": 5,
        "redis": {
               "dsn":"127.0.0.1:6379",
               "password":"MBTWaNOTW35lhjYG0tJy4A==",
               "maxIdle": 10,
               "rsPrefix": "hipaas_rs_",
               "cnamePrefix": "hipaas_cname_"
        },
        "db": {
               "dsn": "@tcp(127.0.0.1:3306)/hipaas?loc=Local&parseTime=true",
               "userName":"root",
               "password":"cuvEg9NGVj15MsFBSuE2ng==",
               "maxIdle": 2
        },
        "mongodb": {
               "url": "mongodb://localhost:27017",
               "db": "hipaas",
               "collector": "hipaas_"
        },
        "netservice": {
               "ip": "127.0.0.1",
               "port": 8889
        },
        "http": {
               "addr": "0.0.0.0",
               "port": 1980
        },
        "rpc": {
               "addr": "0.0.0.0",
               "port": 1970
        },
        "signMsg": {
               "switch": true,
               "interval": 300
        },
        "monitor": {
               "collection": "HIPAAS_MONITOR",
               "deleteFlag": "false",
               "urlFilePath": "/opt/zkClient/message.url",
               "netInterval":60,
               "vmInterval":300
        },
        "health": {
               "timeout": 10,
               "interval": 5
        },
        "maintenancemode":  {
               "switch": false
        },
        "route": {
               "backendroutedrivers": ["HAE_REST","consul"],
               "pushinterval": 5
        },
        "consulserver": {
               "consuladdress": "127.0.0.1:8500",
               "consulscheme": "http"
        }
}
 
```
Some important parameters are explained as follows:
 
- **debug**: debug mode, set "true/false"
- **interval**:time interval of comparing the database
- **dockerPort**:the docker port
- **AgentRpcPort**: the port minion as the RPC server listened
- **localIp**: local IP address of virtual machine
- **businessNIC**:business network card, which is configured by the user
- **redis**: some configuration information for redis
- **health-interval**: Heartbeat cycle, unit is second
- **health-timeout**: The timeout period for the connection server, the unit is millisecond
