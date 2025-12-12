/*******************************************************************************
*  internal/registry/registry.go
*
*  This module provides a descriptor registry for protobuf messages that can be
*  loaded into the logging facility to allow the user custom decoding of
*  protobuf messages in the front-end application.
*******************************************************************************/

package registry

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

/*******************************************************************************
*  TYPES
*******************************************************************************/

type Registry struct {
	files *protoregistry.Files
}

/*******************************************************************************
*  FUNCTIONS
*******************************************************************************/

// NewFromFile loads a single FileDescriptorSet from the given path.
// (Kept for backwards compatibility.)
func NewFromFile(path string) (*Registry, error) {
	return NewFromFiles([]string{path})
}

// NewFromFiles loads and merges multiple FileDescriptorSet files into one registry.
func NewFromFiles(paths []string) (*Registry, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no descriptor paths provided")
	}

	reg := &protoregistry.Files{}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read descriptor set %q: %w", path, err)
		}

		var fds descriptorpb.FileDescriptorSet
		if err := proto.Unmarshal(data, &fds); err != nil {
			return nil, fmt.Errorf("unmarshal descriptor set %q: %w", path, err)
		}

		for _, fdProto := range fds.File {
			fd, err := protodesc.NewFile(fdProto, reg)
			if err != nil {
				return nil, fmt.Errorf("protodesc.NewFile(%s): %w", fdProto.GetName(), err)
			}
			if err := reg.RegisterFile(fd); err != nil {
				// It is usually safe to ignore "already registered" errors if you expect overlaps,
				// but here we fail loudly to surface configuration issues.
				return nil, fmt.Errorf("register file %s: %w", fd.Path(), err)
			}
		}
	}

	return &Registry{files: reg}, nil
}

// NewFromDir loads all *.desc files in the given directory into a single registry.
func NewFromDir(dir string) (*Registry, error) {
	var paths []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".desc" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk dir %q: %w", dir, err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no .desc files found in %q", dir)
	}

	return NewFromFiles(paths)
}

// FormatJSON parses the given payload as the given full type name and returns JSON bytes.
func (r *Registry) FormatJSON(typeName string, payload []byte) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("registry is nil")
	}
	if typeName == "" {
		return nil, fmt.Errorf("empty type name")
	}

	desc, err := r.files.FindDescriptorByName(protoreflect.FullName(typeName))
	if err != nil {
		return nil, fmt.Errorf("find descriptor %q: %w", typeName, err)
	}

	msgDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, fmt.Errorf("descriptor %q is not a message", typeName)
	}

	msg := dynamicpb.NewMessage(msgDesc)
	if err := proto.Unmarshal(payload, msg); err != nil {
		return nil, fmt.Errorf("unmarshal payload as %q: %w", typeName, err)
	}

	opts := protojson.MarshalOptions{
		Multiline:       false,
		Indent:          "",
		EmitUnpopulated: false,
		UseProtoNames:   true,
	}
	return opts.Marshal(msg)
}
