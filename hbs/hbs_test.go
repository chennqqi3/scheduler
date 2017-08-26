package hbs_test
 
import(
       "fmt"
       // "net"
       "net/rpc"
       "time"
       // "net/http"
       . "github.com/onsi/ginkgo"
       . "github.com/onsi/gomega"
       "github-beta.huawei.com/hipaas/scheduler/hbs"
       "github-beta.huawei.com/hipaas/scheduler/g"
       storagenode "github-beta.huawei.com/hipaas/common/storage/node"
       "github-beta.huawei.com/hipaas/common/storage/realstate"
       "github-beta.huawei.com/hipaas/common/redisoperate"
)
 
var _ = Describe("Hbs test",func () {
       const testcfg = "../cfg.example.json"
 
       BeforeEach(func () {
              g.FlagInit()
              err := g.ParseConfig(testcfg)
              if err != nil {
                     Fail("g.ParseConfig fail: " err.Error())
              }
			  g.RedisConnPool,err = redisoperate.InitRedisConnPool(g.Config().Redis)
              if err != nil {
                     Fail("g.RedisConnPool InitRedisConnPool fail: " err.Error())
              }
 
 
              if err = g.NewDbMgr(); nil != err {
                     Fail("Init mysql conn pool fail: " err.Error())
              }
 
              driver := "redis"
              g.RealState,err  = realstate.NewSafeRealState(driver, g.RedisConnPool)
              if err != nil {
                     Fail("RealState init failed")
              } 
              g.RealNodeState,err = storagenode.NewSafeNodeState(driver, g.RedisConnPool)
              if err != nil {
                     Fail("RealNodeState init failed")
              }
			  g.RealMinionState,err = realstate.NewSafeRealState(driver, g.RedisConnPool)
              if err != nil {
                     Fail("RealMinionState init failed")
              }            
       })
       AfterEach(func () {
             
       })
 
       It("RPC service test",func () {
              By("Step 1,When normal",func () {
                     go hbs.Start()
 
                     time.Sleep(time.Duration(2) * time.Second)
 
                     tmpRPCAddr := fmt.Sprintf("%s:%d", g.Config().RPC.Addr, g.Config().RPC.Port)
                     client, err := rpc.Dial("tcp", tmpRPCAddr)
                     Expect(err).NotTo(HaveOccurred())
                     Expect(client).NotTo(BeNil())
					 var response storagenode.NodeResponse
                     err = client.Call("NodeState.NodeDown", "127.0.0.1", &response)              
                     Expect(err).NotTo(HaveOccurred())
                     Expect(response.Code).To(Equal(0))
              })
              // By("Step 2,When Listen failed",func () {
              //    go hbs.Start()
                    
              //    time.Sleep(time.Duration(2) * time.Second)
                    
              //    tmpRPCAddr := fmt.Sprintf("%s:%d", g.Config().RPC.Addr, g.Config().RPC.Port)
              //    client, err := rpc.Dial("tcp", tmpRPCAddr)
              //    Expect(err).To(HaveOccurred())
              //    Expect(client).To(BeNil())
              // })        
       })
})
