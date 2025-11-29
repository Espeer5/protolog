/*******************************************************************************
*  internal/registry/registry.go
*
*  This module provides a descriptor registry for protobuf messages that can be
*  loaded into the logging facility to allow the user custom decoding of
*  protobuf messages in the front-end application.
*******************************************************************************/

package registry

/*******************************************************************************
*  IMPORTS
*******************************************************************************/

import (
	"fmt"
	"os"

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

// NewFromFile loads a FileDescriptorSet from the given path
func NewFromFile(path string) (*Registry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read descriptor set: %w", err)
	}

	var fds descriptorpb.FileDescriptorSet
	if err := proto.Unmarshal(data, &fds); err != nil {
		return nil, fmt.Errorf("unmarshal descriptor set: %w", err)
	}

	files, err := protodesc.NewFiles(&fds)
	if err != nil {
		return nil, fmt.Errorf("build files registry: %w", err)
	}

	return &Registry{files: files}, nil
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
