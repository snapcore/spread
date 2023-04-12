package spread_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/snapcore/spread/spread"

	. "gopkg.in/check.v1"
)

type qemuSuite struct {
	cleanups []func()
}

func makeMockQemuImg(c *C, mockSystemName string) (restore func()) {
	tmpdir := c.MkDir()

	mockQemuDir := filepath.Join(tmpdir, ".spread/qemu")
	err := os.MkdirAll(mockQemuDir, 0755)
	c.Assert(err, IsNil)

	err = ioutil.WriteFile(filepath.Join(mockQemuDir, mockSystemName+".img"), nil, 0644)
	c.Assert(err, IsNil)

	realHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpdir)
	return func() {
		os.Setenv("HOME", realHome)
	}
}

var _ = Suite(&qemuSuite{})

func (s *qemuSuite) SetUpTest(c *C) {
	// SPREAD_QEMU_FALLBACK_BIOS_PATH must not be unset for the tests
	if ovmfEnv, isSet := os.LookupEnv("SPREAD_QEMU_FALLBACK_BIOS_PATH"); isSet {
		os.Unsetenv("SPREAD_QEMU_FALLBACK_BIOS_PATH")
		s.AddCleanup(func() {
			os.Setenv("SPREAD_QEMU_FALLBACK_BIOS_PATH", ovmfEnv)
		})
	}
}

func (s *qemuSuite) TearDownTest(c *C) {
	for _, f := range s.cleanups {
		f()
	}
}

func (s *qemuSuite) AddCleanup(f func()) {
	s.cleanups = append(s.cleanups, f)
}

func (s *qemuSuite) TestQemuCmdWithEfi(c *C) {
	imageName := "ubuntu-20.06-64"

	restore := makeMockQemuImg(c, imageName)
	defer restore()

	tests := []struct {
		BiosSetting       string
		UseBiosQemuOption bool
		expectedErr       string
	}{
		// empty string means legacy
		{"", false, ""},
		{"uefi", true, ""},
		{"invalid", false, `cannot set bios to "invalid", only "uefi" or unset are supported`},
	}

	for _, tc := range tests {
		ms := &spread.System{
			Name:    "some-name",
			Image:   imageName,
			Backend: "qemu",
			Bios:    tc.BiosSetting,
		}
		cmd, err := spread.QemuCmd(ms, "/path/to/image", 512, 9999)
		if tc.expectedErr == "" {
			c.Assert(err, IsNil)
		} else {
			c.Check(err, ErrorMatches, tc.expectedErr)
			continue
		}

		// XXX: reuse testutil.Contains from snapd?
		s := strings.Join(cmd.Args, ":")
		c.Check(strings.Contains(s, ":-bios:/usr/share/OVMF/OVMF_CODE.fd:"), Equals, tc.UseBiosQemuOption)
	}
}

func (s *qemuSuite) TestQemuVirtioDrive(c *C) {
	imageName := "ubuntu-20.06-64"

	restore := makeMockQemuImg(c, imageName)
	defer restore()

	path := "/path/to/image"

	tests := []struct {
		VirtioDisk  bool
		DriveString string
	}{
		{false, fmt.Sprintf("file=%s,format=raw", path)},
		{true, fmt.Sprintf("file=%s,format=raw,if=virtio", path)},
	}

	for _, tc := range tests {
		ms := &spread.System{
			Name:       "some-name",
			Image:      imageName,
			Backend:    "qemu",
			VirtioDisk: tc.VirtioDisk,
		}
		cmd, err := spread.QemuCmd(ms, path, 512, 9999)
		c.Assert(err, IsNil)

		s := strings.Join(cmd.Args, " ")
		c.Assert(s, Matches, fmt.Sprintf("^.*-drive %s.*$", tc.DriveString))
	}
}

func (s *qemuSuite) TestQemuVirtioNet(c *C) {
	imageName := "ubuntu-20.06-64"

	restore := makeMockQemuImg(c, imageName)
	defer restore()

	tests := []struct {
		VirtioNet       bool
		NetDeviceString string
	}{
		{false, fmt.Sprintf("netdev=user0,driver=e1000")},
		{true, fmt.Sprintf("netdev=user0,driver=virtio-net-pci")},
	}

	for _, tc := range tests {
		ms := &spread.System{
			Name:      "some-name",
			Image:     imageName,
			Backend:   "qemu",
			VirtioNet: tc.VirtioNet,
		}
		cmd, err := spread.QemuCmd(ms, "/path/to/image", 512, 9999)
		c.Assert(err, IsNil)

		s := strings.Join(cmd.Args, " ")
		c.Assert(s, Matches, fmt.Sprintf("^.*-device %s.*$", tc.NetDeviceString))
	}
}
