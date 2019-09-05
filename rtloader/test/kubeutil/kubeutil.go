package testkubeutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"unsafe"

	common "github.com/DataDog/datadog-agent/rtloader/test/common"
	"github.com/DataDog/datadog-agent/rtloader/test/helpers"
	yaml "gopkg.in/yaml.v2"
)

// #cgo CFLAGS: -I../../include
// #cgo !windows LDFLAGS: -L../../rtloader/ -ldatadog-agent-rtloader -ldl
// #cgo windows LDFLAGS: -L../../rtloader/ -ldatadog-agent-rtloader -lstdc++ -static
//
// #include <stdlib.h>
// #include <datadog_agent_rtloader.h>
//
// extern void getConnectionInfo(char **);
//
// static void initKubeUtilTests(rtloader_t *rtloader) {
//    set_get_connection_info_cb(rtloader, getConnectionInfo);
// }
import "C"

var (
	rtloader   *C.rtloader_t
	tmpfile    *os.File
	returnNull bool
)

func setUp() error {
	rtloader = (*C.rtloader_t)(common.GetRtLoader())
	if rtloader == nil {
		return fmt.Errorf("make failed")
	}

	// Initialize memory tracking
	helpers.InitMemoryTracker()

	var err error
	tmpfile, err = ioutil.TempFile("", "testout")
	if err != nil {
		return err
	}

	// Updates sys.path so testing Check can be found
	C.add_python_path(rtloader, C.CString("../python"))

	if ok := C.init(rtloader); ok != 1 {
		return fmt.Errorf("`init` failed: %s", C.GoString(C.get_error(rtloader)))
	}

	C.initKubeUtilTests(rtloader)

	return nil
}

func tearDown() {
	os.Remove(tmpfile.Name())
}

func run(call string) (string, error) {
	tmpfile.Truncate(0)
	code := C.CString(fmt.Sprintf(`
try:
	import kubeutil
	%s
except Exception as e:
	with open(r'%s', 'w') as f:
		f.write("{}\n".format(e))
`, call, tmpfile.Name()))

	runtime.LockOSThread()
	state := C.ensure_gil(rtloader)

	ret := C.run_simple_string(rtloader, code) == 1
	C.free(unsafe.Pointer(code))

	C.release_gil(rtloader, state)
	runtime.UnlockOSThread()

	if !ret {
		return "", fmt.Errorf("`run_simple_string` errored")
	}

	output, err := ioutil.ReadFile(tmpfile.Name())

	return string(output), err
}

//export getConnectionInfo
func getConnectionInfo(in **C.char) {
	if returnNull {
		return
	}

	h := map[string]string{
		"FooKey": "FooValue",
		"BarKey": "BarValue",
	}
	retval, _ := yaml.Marshal(h)

	*in = C.CString(string(retval))
}
