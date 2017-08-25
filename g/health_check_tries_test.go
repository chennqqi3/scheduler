package g_test
 
import (
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "github-beta.huawei.com/hipaas/common/redisoperate"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       docker "github.com/fsouza/go-dockerclient"
       storageapp "github-beta.huawei.com/hipaas/common/storage/app"
       "math/rand"
       "time"
)
 
var _ = Describe("Health Check tries",func () {
       var (
              ctnr *realstate.Container
              err  error
       )

randomstr := func(prefix string,lns int) string{
              str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
              bytes := []byte(str)
              result := []byte{}
              r := rand.New(rand.NewSource(time.Now().UnixNano()))
              for i :=0;i < lns; i   {
                     result = append(result,bytes[r.Intn(len(bytes))])
              }
              return prefix string(result)
       }
 
       // prepare action
       g.FlagInit()
       err = g.ParseConfig("../cfg.example.json")
       if err != nil {
              Fail("g.ParseConfig failed: " err.Error())
       }
       g.RedisConnPool, err = redisoperate.InitRedisConnPool(g.Config().Redis)
       if nil != err {
              Fail("InitRedisConnPool fail, err: " err.Error())
       }
	   
	   g.RealState, err = realstate.NewSafeRealState("redis", g.RedisConnPool)
       if err != nil {
              Fail("realstate.NewSafeRealState fail: " err.Error())
       }     
 
       BeforeEach(func () {
              ctnr = &realstate.Container {
                     ID:        randomstr("hcyTestHCCtnr",10),
                     IP:        "127.0.0.1",
                     Image:     "10.63.121.36:5000/official/tomcat:latest",
                     Recovery:  false,
                     AppName:   randomstr("hcyTestHCAppName",10),
                     Ports:     []*realstate.Port{&realstate.Port{PublicPort: 3128}},
                     Status:    "Up 2 minutes",
                     Hostname:  "cloud.huawei.com",
                     Region:    "Net67",
                     VMType:    "was",
                     CPU:       1,
                     Memory:    128,
					 PortInfos: []realstate.PortInfo{realstate.PortInfo{Portname: "public", Ports: []docker.PortBinding{docker.PortBinding{HostIP: "10.63.67.130", HostPort: "3128"}}, OriginPort: "8080"}},
                     AppType:   "LRP",
                     Envs:      []storageapp.Env{storageapp.Env{K: "gopath", V: "/home/go"}},
              }
              g.RealState.UpdateContainer(ctnr)
       })
       AfterEach(func () {
              g.RealState.DeleteContainer(ctnr.AppName, ctnr)
       })
	   
	   Describe("Increase/Delete",func () {
              It("Testcase",func () {
                     By("Step 1,Increase by nil ctnr",func () {
                            result,err := g.HealthCheckTries.Increase(nil)
                            Expect(err).To(HaveOccurred())
                            Expect(result).To(Equal(0))
                     })
                     By("Step 2,Increase by normal",func () {
                            result,err := g.HealthCheckTries.Increase(ctnr)
                            Expect(err).NotTo(HaveOccurred())
                            Expect(result).NotTo(Equal(0))
                     })
					 By("Step 3,Delete by nil ctnr",func () {
                            err = g.HealthCheckTries.Delete(nil)
                            Expect(err).To(HaveOccurred())
                     })
                     By("Step 4,Delete by normal",func () {
                            err = g.HealthCheckTries.Delete(ctnr)
                            Expect(err).NotTo(HaveOccurred())
                     })
              })
       })
})
