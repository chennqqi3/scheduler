package hbs
 
import (
       "fmt"
       "github-beta.huawei.com/hipaas/glog"
       "github-beta.huawei.com/hipaas/scheduler/g"
       "net"
       "net/rpc"
)
 
// Start listen for RPC connections on port g.Config().RPC.Port [1980].
func Start() {
       addr := fmt.Sprintf("%s:%d", g.Config().RPC.Addr, g.Config().RPC.Port)
 
       tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
       if err != nil {
              glog.Fatalf("[FATAL] net.ResolveTCPAddr fail: %s", err)
       }
 
       listener, err := net.ListenTCP("tcp", tcpAddr)
       if err != nil {
              glog.Fatalf("[FATAL] listen %s fail: %s", addr, err)
       }
 
       rpc.Register(new(NodeState))
 
       for {
              conn, err := listener.Accept()
              if err != nil {
                     glog.Errorf("[ERROR] listener.Accept occur error: %s", err)
                     continue
              }
              go rpc.ServeConn(conn)
       }
}
