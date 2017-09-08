package executor_test
 
import (
       storageapp "github-beta.huawei.com/hipaas/common/storage/app"
       // "github-beta.huawei.com/hipaas/common/storage/createstate"
       "github-beta.huawei.com/hipaas/common/storage/node"
       . "github-beta.huawei.com/hipaas/scheduler/executor"
       "github-beta.huawei.com/hipaas/scheduler/g"
 
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
 
       // "sort"
       // "fmt"
       "os"
       "time"
       "math/rand"  
       // "strings"
)
 
// func ParaRunContainerOption(app *app.App, deployInfo *node.DeployInfo) (*createstate.RunContainerOption, error)
// func ParaPath(path string) string
 
var _ = Describe("executor suite test", func() {
       var (
              app        *storageapp.App
              deployInfo *node.DeployInfo
              env            map[string]string
       )
 
       randomstr := func(prefix string,lns int) string{
              str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
              bytes := []byte(str)
              result := []byte{}
              r := rand.New(rand.NewSource(time.Now().UnixNano()))
              for i :=0;i < lns; i++ {
                     result = append(result,bytes[r.Intn(len(bytes))])
              }
              return prefix+string(result)
       }
 
       // g parse config
       g.FlagInit()
 
       g.ParseConfig("../cfg.example.json")
 
       BeforeEach(func(){
 
       })
       AfterEach(func(){
 
       })
 
       Context("ParaPath", func() {
              It("when the length of path is equal zero", func() {
                     path := ""
                     pathstr := ParaPath(path)
                     Expect(pathstr).To(Equal(path))
              })
              It("when the length of path is equal one", func() {
                     path := "/"
                     pathstr := ParaPath(path)
                     Expect(pathstr).To(Equal(path))
              })
              It("when the length of path is more than one and the last string is '/'", func() {
                     path := "/tmp/testdir1/"
                     pathstr := ParaPath(path)
                     Expect(pathstr).To(Equal(path[:len(path)-1]))
              })
       })
       Context("ParaRunContainerOption", func() {
              JustBeforeEach(func() {
                     app = &storageapp.App{
                            Name:        randomstr("hcytest",10),
                            Id:           int64(os.Getpid()),
                            CPU:         1,
                            Memory:      256,
                            Instance: 1,
                            Image: storageapp.Image{
                                   DockerImageURL: "9.91.17.12:5000/harlan/test",
                                   DockerLoginServer: "",
                                   DockerUser:        "",
                                   DockerPassword:    "",
                                   DockerEmail:       "wujianlin1@huawei.com",
                            },
                            Recovery: 0,
                            Status:   "Started",
                            Region:   "regiontest",
                            VMType:   "container",
                            Hostnames:    []storageapp.Hostname{storageapp.Hostname{Hostname: "zhangmanjuan", Subdomain: "huawei.com", Status: "1"}},
                            Ports: []storageapp.Port{
                                   {
                                          PortName: "public",
                                          Port: 8080,
                                   },
                                   {
                                          PortName: "monitor",
                                          Port: 9090,
                                   },
                            },
                            Mount: []storageapp.Mount {
                                   {
                                          HPath: "/testdir123232",
                                          CPath: "/testdir123432",
                                          Type: "private",
                                          Copy: "copy",
                                          Source: "/test/dir",
                                          User: "testharlan",
                                          Privilege: "test",
                                   },
                            },           
                     }
                     env = map[string]string{
                            "HIPAAS_IP":"10.62.12.23",
                            "HIPAAS_GATEWAY":"10.10.1.1",
                            "HIPAAS_MASK":"255.0.0.0",
                            "HIPAAS_CPU":"1",
                            "HIPAAS_MEMORY":"256",
                            "HIPAAS_APPNAME":app.Name,
                            "HIPAAS_RECOVERY":"false",
                            "HIPAAS_HOSTNAME":"zhangmanjuan",
                            "HIPAAS_PORTS":"",
                            "HIPAAS_APPTYPE":"",
                            "HIPAAS_ENVS":"",
                            "HIPAAS_CTNRARCHIVE":"",
                     }
                     for key,val := range env {
                            app.Envs = append(app.Envs,storageapp.Env{
                                   K: key,
                                   V: val,
                            })
                     }                   
                     deployInfo = &node.DeployInfo{
                            AgentIP:     "127.0.0.1",
                            Region:      "regiontest",
                            Hostname:    "zhangmanjuan",
                            Ctnrname:     randomstr("ctnrtest",10),
                            ContainerIP: "10.26.12.23",
                            Gateway:     "10.10.11.1",
                            Mask:        "255.0.0.0",
                            Recovery:    "false",
                     }
              })    
              It("Image error",func(){
                     app.Image.DockerImageURL = "9.91.12.33/harlan/test"
                     app.Image.DockerLoginServer = "9.91.34.54"
                     app.Image.DockerUser = "harlantest"
                     app.Image.DockerPassword = "testpasswd"
 
                     _,err := ParaRunContainerOption(app,deployInfo)
                     Expect(err).To(HaveOccurred())
              })
              It("Mount error",func(){
                     app.Recovery = 1
                     app.Mount = []storageapp.Mount {
                            {
                                   HPath: "testdir123232",
                                   CPath: "testdir123432",
                                   Type: "private",
                                   Copy: "copy",
                                   Source: "",
                                   User: "",
                                   Privilege: "test",
                            },
                     }
 
                     _,err := ParaRunContainerOption(app,deployInfo)
                     Expect(err).To(HaveOccurred())
              })
              It("normal testcase",func(){
                     result,err := ParaRunContainerOption(app,deployInfo)
                     Expect(err).NotTo(HaveOccurred())
                     _ = result
              })    
       })
})
