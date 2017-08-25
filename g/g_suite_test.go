package g_test
 
import (
       . "github.com/onsi/ginkgo"
       "github.com/onsi/ginkgo/reporters"
       . "github.com/onsi/gomega"
       "testing"
       "time"
)
 
var waitTime time.Duration = 2
 
func TestG(t *testing.T) {
       RegisterFailHandler(Fail)
       junitReporter := reporters.NewJUnitReporter("g.junit.xml")
       RunSpecsWithDefaultAndCustomReporters(t, "g suite", []Reporter{junitReporter})
       //RunSpecs(t, "G Suite")
}
