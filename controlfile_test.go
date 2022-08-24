package down

import (
	"fmt"
	"io"
	"os"
	"testing"
)

func TestControlFile(t *testing.T) {
	cf := newControlfile(1)
	out := cf.Encoding()
	fmt.Printf("%#+v \n", out.Len())
}

func TestReadControlFile(t *testing.T) {
	f, err := os.Open("/Users/rockrabbit/projects/down/tmp/photo-1661179738571-82e1815d83bb.jpg.down")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	cf := readControlfile(data)

	fmt.Printf("%v\n", cf)
	for _, v := range cf.threadblock {
		fmt.Printf("%v\n", v.completedLength)
	}

}
