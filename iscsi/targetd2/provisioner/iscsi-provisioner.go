/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package provisioner

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var log = logrus.New()

type iscsiProvisioner struct{}

// NewiscsiProvisioner creates new iscsi provisioner
func NewiscsiProvisioner() controller.Provisioner {

	initLog()
	return &iscsiProvisioner{}

}

// getAccessModes returns access modes iscsi volume supported.
func (p *iscsiProvisioner) getAccessModes() []v1.PersistentVolumeAccessMode {
	return []v1.PersistentVolumeAccessMode{
		v1.ReadWriteOnce,
		v1.ReadOnlyMany,
	}
}

// Provision creates a storage asset and returns a PV object representing it.
func (p *iscsiProvisioner) Provision(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	if !util.AccessModesContainedInAll(p.getAccessModes(), options.PVC.Spec.AccessModes) {
		return nil, fmt.Errorf("invalid AccessModes %v: only AccessModes %v are supported", options.PVC.Spec.AccessModes, p.getAccessModes())
	}
	log.Debugln("new provision request received for pvc: ", options.PVName)
	vol, path, err := p.createVolume(options)
	if err != nil {
		log.Warnln(err)
		return nil, err
	}
	log.Debugln("volume created with vol and lun: ", vol, path)

	annotations := make(map[string]string)
	annotations["volume_name"] = vol
	annotations["path"] = path

	items := strings.Split(path, "/")
	lunname := items[len(items)-1]
	lun, _ := strconv.Atoi(strings.TrimPrefix(lunname, "lun"))

	targetPortal := fmt.Sprintf("%s:%s", os.Getenv("TARGET_IP"), os.Getenv("TARGET_PORT"))

	var pv *v1.PersistentVolume
	if string(*options.PVC.Spec.VolumeMode) == "" || strings.ToLower(string(*options.PVC.Spec.VolumeMode)) == "filesystem" {
		fsType := getFsType(options.Parameters["fsType"])
		if fsType == "" {
			fsType = "xfs"
		}
		pv = &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:        options.PVName,
				Labels:      map[string]string{},
				Annotations: annotations,
			},
			Spec: v1.PersistentVolumeSpec{
				PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
				AccessModes:                   options.PVC.Spec.AccessModes,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
				},
				// set volumeMode from PVC Spec
				VolumeMode: options.PVC.Spec.VolumeMode,
				PersistentVolumeSource: v1.PersistentVolumeSource{
					ISCSI: &v1.ISCSIPersistentVolumeSource{
						TargetPortal: targetPortal,
						IQN:          options.Parameters["iqn"],
						Lun:          int32(lun),
						ReadOnly:     getReadOnly(options.Parameters["readonly"]),
						FSType:       fsType,
					},
				},
			},
		}
	} else {
		pv = &v1.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name:        options.PVName,
				Labels:      map[string]string{},
				Annotations: annotations,
			},
			Spec: v1.PersistentVolumeSpec{
				PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
				AccessModes:                   options.PVC.Spec.AccessModes,
				Capacity: v1.ResourceList{
					v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
				},
				// set volumeMode from PVC Spec
				VolumeMode: options.PVC.Spec.VolumeMode,
				PersistentVolumeSource: v1.PersistentVolumeSource{
					ISCSI: &v1.ISCSIPersistentVolumeSource{
						TargetPortal: targetPortal,
						IQN:          options.Parameters["iqn"],
						Lun:          int32(lun),
						ReadOnly:     getReadOnly(options.Parameters["readonly"]),
					},
				},
			},
		}
	}
	return pv, nil
}

func getReadOnly(readonly string) bool {
	isReadOnly, err := strconv.ParseBool(readonly)
	if err != nil {
		return false
	}
	return isReadOnly
}

func getFsType(fsType string) string {
	if fsType == "" {
		return viper.GetString("ext4")
	}
	return fsType
}

// Delete removes the storage asset that was created by Provision represented
// by the given PV.
func (p *iscsiProvisioner) Delete(volume *v1.PersistentVolume) error {
	//vol from the annotation
	vol := volume.GetName()
	path := volume.Annotations["path"]
	lpath := getPath()
	log.Debugln("volume deletion request received: ", vol)
	log.Debugln("removing logical volume : ", vol, path)

	items := strings.Split(path, "/")
	lun := items[len(items)-1]
	dir := strings.TrimSuffix(path, fmt.Sprintf("/%s", lun))

	//targetcli /iscsi/${targetName}/tpg1/luns delete ${lun}
	out, err := exec.Command("sh", "-c", fmt.Sprintf("targetcli %s delete %s", dir, lun)).Output()
	if err != nil {
		log.Errorln("Delete lun failed with path, lun, err, output: ", dir, lun, err, out)
		return err
	}

	//targetcli /backstores/fileio delete ${volumeName}
	out, err = exec.Command("sh", "-c", fmt.Sprintf("targetcli /backstores/fileio delete %s", vol)).Output()
	if err != nil {
		log.Errorln("Delete fileio backstore failed with name, err, output: ", vol, err, out)
		return err
	}

	out, err = exec.Command("sh", "-c", fmt.Sprintf("rm -f /%s/%s", lpath, vol)).Output()
	if err != nil {
		log.Errorln("Delete disk file failed with name, err, output: ", lpath, vol, err, out)
		return err
	}

	log.Debugln("Delete logical volume successfully: ", volume.Annotations["volume_name"], volume.Annotations["path"])
	return nil
}

func initLog() {
	var err error
	log.Level, err = logrus.ParseLevel(viper.GetString("log-level"))
	if err != nil {
		log.Fatalln(err)
	}
}

func initTarget(options controller.VolumeOptions) error {

	log.Debugln("Start to initialize iscsi target")

	targetName := getTargetName(options)
	//targetcli /iscsi ls {targetName}
	out, err := exec.Command("sh", "-c", fmt.Sprintf("targetcli /iscsi ls %s 2>&1", targetName)).Output()
	if err != nil && strings.Contains(string(out), "No such path") {
		createTarget(options)
		return nil
	} else if err != nil {
		log.Fatalln("Get target failed with targetName, err, output: ", targetName, err, out)
		return err
	}

	return nil
}

func createTarget(options controller.VolumeOptions) error {

	targetName := getTargetName(options)

	//targetcli /iscsi create ${targetName}
	out, err := exec.Command("sh", "-c", fmt.Sprintf("targetcli /iscsi create %s", targetName)).Output()
	if err != nil {
		log.Errorln("Create target failed with targetName, err, output: ", targetName, err, out)
		return err
	}

	//targetcli /iscsi/${targetName}/tpg1/portals delete 0.0.0.0 3260
	out, err = exec.Command("sh", "-c", fmt.Sprintf("targetcli /iscsi/%s/tpg1/portals delete 0.0.0.0 3260 2>&1", targetName)).Output()
	if err != nil && !strings.Contains(string(out), "No such NetworkPortal") {
		log.Errorln("Delete prtals(0.0.0.0:3260) for target failed with targetName, err, output: ", targetName, err, out)
		return err
	}

	//targetcli /iscsi/${targetName}/tpg1 set attribute generate_node_acls=1
	out, err = exec.Command("sh", "-c", fmt.Sprintf("targetcli /iscsi/%s/tpg1 set attribute generate_node_acls=1", targetName)).Output()
	if err != nil {
		log.Errorln("Set generate_node_acls attribute for target failed with targetName, err, output: ", targetName, err, out)
		return err
	}

	//targetcli /iscsi/${targetName}/tpg1 set demo_mode_write_protect=0
	out, err = exec.Command("sh", "-c", fmt.Sprintf("targetcli /iscsi/%s/tpg1 set attribute demo_mode_write_protect=0", targetName)).Output()
	if err != nil {
		log.Errorln("Set demo_mode_write_protec attribute for target failed with targetName, err, output: ", targetName, err, out)
		return err
	}

	// targetcli /iscsi/${TARGET_NAME}/tpg1/portals create ${targetIP}
	out, err = exec.Command("sh", "-c", fmt.Sprintf("targetcli /iscsi/%s/tpg1/portals create %s %s", targetName, os.Getenv("TARGET_IP"), os.Getenv("TARGET_PORT"))).Output()
	if err != nil {
		log.Errorln("Create portal for target failed with portal, targetName, err, output: ", os.Getenv("TARGET_IP"), os.Getenv("TARGET_PORT"), targetName, err, out)
		return err
	}

	return nil
}

func (p *iscsiProvisioner) createVolume(options controller.VolumeOptions) (vol string, path string, err error) {
	size := getSize(options)
	lpath := getPath()
	targetName := getTargetName(options)
	vol = p.getVolumeName(options)

	initTarget(options)
	log.Debugln("creating volume name, size: ", vol, size)

	//targetcli /backstores/fileio create ${volumeName} /${path}/${volumeName} size
	out, err := exec.Command("sh", "-c", fmt.Sprintf("targetcli /backstores/fileio create %s %s/%s %d", vol, lpath, vol, size)).Output()
	if err != nil {
		log.Errorln("Create fileio backstore failed with volumename, path, size, err, output: ", vol, lpath, size, err, out)
		return vol, path, err
	}

	//targetcli /iscsi/${targetName}/tpg1/luns create /backstores/fileio/${volumeName}
	out, err = exec.Command("sh", "-c", fmt.Sprintf("targetcli /iscsi/%s/tpg1/luns create /backstores/fileio/%s", targetName, vol)).Output()
	if err != nil {
		log.Errorln("Create lun for target failed with targetName, volume, err, output: ", targetName, vol, err, out)
		return vol, path, err
	}

	//targetcli /iscsi/${targetName}/tpg1/luns ls|grep ${volumeName}|awk {'print $2'}
	out, err = exec.Command("sh", "-c", fmt.Sprintf("targetcli /iscsi/%s/tpg1/luns ls|grep %s|awk {'print $2'}", targetName, vol)).Output()
	if err != nil {
		log.Errorln("Get lun num from target failed with targetName, volumename, err, output: ", targetName, vol, err, out)
		return vol, path, err
	}
	path = fmt.Sprintf("/iscsi/%s/tpg1/luns/%s", targetName, out)
	log.Debugln("Volume created successfully in the path: ", path)
	return vol, path, nil
}

func getTargetName(options controller.VolumeOptions) string {
	return options.Parameters["iqn"]
}

func getPortals(options controller.VolumeOptions) []string {
	var portals []string
	if len(options.Parameters["portals"]) > 0 {
		portals = strings.Split(options.Parameters["portals"], ",")
	}
	return portals
}

func getSize(options controller.VolumeOptions) int64 {
	q := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	return q.Value()
}

func getPath() string {
	return "/iscsi_disks"
}

func (p *iscsiProvisioner) getVolumeName(options controller.VolumeOptions) string {
	return options.PVName
}

func (p *iscsiProvisioner) SupportsBlock() bool {
	return true
}
