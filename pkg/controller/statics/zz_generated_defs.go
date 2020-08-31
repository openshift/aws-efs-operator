// Code generated for package statics by go-bindata DO NOT EDIT. (@generated)
// sources:
// defs/csidriver.yaml
// defs/daemonset.yaml
// defs/scc.yaml
// defs/serviceaccount.yaml
// defs/storageclass.yaml
package statics

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)
type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _defsCsidriverYaml = []byte(`# Source: https://github.com/kubernetes-sigs/aws-efs-csi-driver/blob/51d19a433dcfc47fbb7b7a0e1c8ff6ab98ce87e9/deploy/kubernetes/base/csidriver.yaml
kind: CSIDriver
apiVersion: storage.k8s.io/v1beta1
metadata:
  name: efs.csi.aws.com
spec:
  attachRequired: false
  podInfoOnMount: false
  volumeLifecycleModes:
    - Persistent
`)

func defsCsidriverYamlBytes() ([]byte, error) {
	return _defsCsidriverYaml, nil
}

func defsCsidriverYaml() (*asset, error) {
	bytes, err := defsCsidriverYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "defs/csidriver.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _defsDaemonsetYaml = []byte(`# Source: https://raw.githubusercontent.com/kubernetes-sigs/aws-efs-csi-driver/51d19a433dcfc47fbb7b7a0e1c8ff6ab98ce87e9/deploy/kubernetes/base/node.yaml
# Changes tagged with DELTA: comments
kind: DaemonSet
apiVersion: apps/v1
metadata:
  name: efs-csi-node
  # DELTA: Use a custom namespace rather than kube-system
  # The namespace is populated dynamically by the operator.
spec:
  selector:
    matchLabels:
      app: efs-csi-node
  template:
    metadata:
      labels:
        app: efs-csi-node
    spec:
      # DELTA: Added
      serviceAccountName: efs-csi-sa
      # DELTA: Removed
      # priorityClassName: system-node-critical
      nodeSelector:
        kubernetes.io/os: linux
        kubernetes.io/arch: amd64
        # DELTA: only deploy this on worker nodes
        # NOTE: This will hit infra nodes as well.
        node-role.kubernetes.io/worker: ''
      hostNetwork: true
      dnsPolicy: ClusterFirstWithHostNet
      tolerations:
        - operator: Exists
      containers:
        - name: efs-plugin
          securityContext:
            privileged: true
          # DELTA: fq image
          # TODO(efried): Pin to a release
          #  https://github.com/kubernetes-sigs/aws-efs-csi-driver/issues/152
          # For now, freeze to a known working commit tag
          image: registry.hub.docker.com/amazon/aws-efs-csi-driver:778131e
          # DELTA: Always pull
          imagePullPolicy: Always
          args:
            - --endpoint=$(CSI_ENDPOINT)
            - --logtostderr
            - --v=5
          env:
            - name: CSI_ENDPOINT
              value: unix:/csi/csi.sock
          volumeMounts:
            - name: kubelet-dir
              mountPath: /var/lib/kubelet
              mountPropagation: "Bidirectional"
            - name: plugin-dir
              mountPath: /csi
            - name: efs-state-dir
              mountPath: /var/run/efs
          ports:
            - containerPort: 9809
              hostPort: 9809
              name: healthz
              protocol: TCP
          livenessProbe:
            httpGet:
              path: /healthz
              port: healthz
            initialDelaySeconds: 10
            timeoutSeconds: 3
            periodSeconds: 2
            failureThreshold: 5
        - name: csi-driver-registrar
          image: quay.io/k8scsi/csi-node-driver-registrar:v1.3.0
          # DELTA: Always pull
          imagePullPolicy: Always
          args:
            - --csi-address=$(ADDRESS)
            - --kubelet-registration-path=$(DRIVER_REG_SOCK_PATH)
            - --v=5
          env:
            - name: ADDRESS
              value: /csi/csi.sock
            - name: DRIVER_REG_SOCK_PATH
              value: /var/lib/kubelet/plugins/efs.csi.aws.com/csi.sock
            - name: KUBE_NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          volumeMounts:
            - name: plugin-dir
              mountPath: /csi
            - name: registration-dir
              mountPath: /registration
        - name: liveness-probe
          imagePullPolicy: Always
          image: quay.io/k8scsi/livenessprobe:v2.0.0
          args:
            - --csi-address=/csi/csi.sock
            - --health-port=9809
          volumeMounts:
            - mountPath: /csi
              name: plugin-dir
      volumes:
        - name: kubelet-dir
          hostPath:
            path: /var/lib/kubelet
            type: Directory
        - name: registration-dir
          hostPath:
            path: /var/lib/kubelet/plugins_registry/
            type: Directory
        - name: plugin-dir
          hostPath:
            path: /var/lib/kubelet/plugins/efs.csi.aws.com/
            type: DirectoryOrCreate
        - name: efs-state-dir
          hostPath:
            path: /var/run/efs
            type: DirectoryOrCreate
`)

func defsDaemonsetYamlBytes() ([]byte, error) {
	return _defsDaemonsetYaml, nil
}

func defsDaemonsetYaml() (*asset, error) {
	bytes, err := defsDaemonsetYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "defs/daemonset.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _defsSccYaml = []byte(`allowHostDirVolumePlugin: true
allowHostIPC: true
allowHostNetwork: true
allowHostPID: true
allowHostPorts: true
allowPrivilegeEscalation: true
allowPrivilegedContainer: true
allowedCapabilities:
- '*'
allowedUnsafeSysctls:
- '*'
apiVersion: security.openshift.io/v1
fsGroup:
  type: RunAsAny
groups:
- system:cluster-admins
- system:nodes
- system:masters
kind: SecurityContextConstraints
metadata:
  annotations:
    kubernetes.io/description: 'Highly privileged SCC for the EFS CSI driver DaemonSet.'
  name: efs-csi-scc
readOnlyRootFilesystem: false
runAsUser:
  type: RunAsAny
seLinuxContext:
  type: RunAsAny
seccompProfiles:
- '*'
supplementalGroups:
  type: RunAsAny
users:
- system:admin
# The operator must add:
# - system:serviceaccount:${namespace}:${serviceaccount}
volumes:
- '*'
`)

func defsSccYamlBytes() ([]byte, error) {
	return _defsSccYaml, nil
}

func defsSccYaml() (*asset, error) {
	bytes, err := defsSccYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "defs/scc.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _defsServiceaccountYaml = []byte(`# Privileged service account for the EFS CSI driver's DaemonSet
apiVersion: v1
kind: ServiceAccount
metadata:
  name: efs-csi-sa
  # NOTE: namespace is set dynamically after this is loaded.
`)

func defsServiceaccountYamlBytes() ([]byte, error) {
	return _defsServiceaccountYaml, nil
}

func defsServiceaccountYaml() (*asset, error) {
	bytes, err := defsServiceaccountYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "defs/serviceaccount.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _defsStorageclassYaml = []byte(`---
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: efs-sc
provisioner: efs.csi.aws.com
reclaimPolicy: Delete
volumeBindingMode: Immediate
`)

func defsStorageclassYamlBytes() ([]byte, error) {
	return _defsStorageclassYaml, nil
}

func defsStorageclassYaml() (*asset, error) {
	bytes, err := defsStorageclassYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "defs/storageclass.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"defs/csidriver.yaml":      defsCsidriverYaml,
	"defs/daemonset.yaml":      defsDaemonsetYaml,
	"defs/scc.yaml":            defsSccYaml,
	"defs/serviceaccount.yaml": defsServiceaccountYaml,
	"defs/storageclass.yaml":   defsStorageclassYaml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"defs": &bintree{nil, map[string]*bintree{
		"csidriver.yaml":      &bintree{defsCsidriverYaml, map[string]*bintree{}},
		"daemonset.yaml":      &bintree{defsDaemonsetYaml, map[string]*bintree{}},
		"scc.yaml":            &bintree{defsSccYaml, map[string]*bintree{}},
		"serviceaccount.yaml": &bintree{defsServiceaccountYaml, map[string]*bintree{}},
		"storageclass.yaml":   &bintree{defsStorageclassYaml, map[string]*bintree{}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
