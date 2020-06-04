package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/efs"
)

const (
	fsTokenMarker = "managed.openshift.io/aws-efs-operator-fs"
	apTokenMarker = "managed.openshift.io/aws-efs-operator-ap"
)

// TODO(efried): real log setup
type logstub struct{}

func (l *logstub) Info(msg string, args ...interface{}) {
	fmt.Println(msg)
	for i := 0; i < len(args); i += 2 {
		fmt.Printf("\t%s: %s\n", args[i], args[i+1])
	}
}

var log = logstub{}

// accessPoint is a map of an access point "key" (arbitrary user-specified name) to its AccessPointId.
type accessPoints map[string]string

type fileSystem struct {
	fileSystemID       string
	lastLifeCycleState string
	accessPoints       accessPoints
}

// fileSystems is a map of a file system "key" (arbitrary user-specified name) to a fileSystem struct
// which contains its FSID and access points.
type fileSystems map[string]fileSystem

func getSession() *session.Session {
	return session.Must(session.NewSessionWithOptions(
		session.Options{
			SharedConfigState: session.SharedConfigEnable}))
}

func getEC2(sess *session.Session) *ec2.EC2 {
	return ec2.New(sess)
}

func getEFS(sess *session.Session) *efs.EFS {
	return efs.New(sess)
}

func getWorkers(ec2svc *ec2.EC2) []*ec2.Instance {
	filt := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			{
				// TODO(efried): Is this the right filter?
				Name:   aws.String("iam-instance-profile.arn"),
				Values: []*string{aws.String("*-worker-*")},
			},
		},
	}
	res, err := ec2svc.DescribeInstances(filt)
	if err != nil {
		panic(err)
	}
	ret := make([]*ec2.Instance, 0)
	for _, reservation := range res.Reservations {
		for _, inst := range reservation.Instances {
			ret = append(ret, inst)
		}
	}
	if len(ret) == 0 {
		panic("Couldn't find any workers.")
	}
	return ret
}

func getSecurityGroupID(workers []*ec2.Instance) string {
	// The security group ought to be the same for any worker, so just pick the first one
	return *workers[0].SecurityGroups[0].GroupId
}

func getSubnetIDs(workers []*ec2.Instance) []*string {
	ret := make([]*string, 0)
	for _, inst := range workers {
		ret = append(ret, inst.NetworkInterfaces[0].SubnetId)
	}
	if len(ret) == 0 {
		panic("No subnets found.")
	}
	return ret
}

func getOwnedTag(workers []*ec2.Instance) *ec2.Tag {
	// The owned tag should be the same for any instance, so just pick the first one
	for _, tag := range workers[0].Tags {
		if *tag.Value == "owned" {
			return tag
		}
	}
	panic("Couldn't find an 'owned' tag.")
}

func tagEC2ToEFS(ec2Tag *ec2.Tag) *efs.Tag {
	return &efs.Tag{
		Key:   ec2Tag.Key,
		Value: ec2Tag.Value,
	}
}

func ensureNFSIngressRule(ec2svc *ec2.EC2, sgid string) {
	dsgInput := &ec2.DescribeSecurityGroupsInput{
		GroupIds: []*string{&sgid},
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("ip-permission.to-port"),
				Values: []*string{aws.String("2049")},
			},
		},
	}
	sgs, err := ec2svc.DescribeSecurityGroups(dsgInput)
	if err != nil {
		panic(err)
	}
	if len(sgs.SecurityGroups) != 0 {
		log.Info("NFS ingress rule already exists; skipping.")
		return
	}

	log.Info("Creating NFS ingress rule", "security group ID", sgid)
	asgiInput := &ec2.AuthorizeSecurityGroupIngressInput{
		// TODO(efried): Is this right, or too permissive?
		CidrIp:     aws.String("0.0.0.0/0"),
		FromPort:   aws.Int64(2049),
		GroupId:    aws.String(sgid),
		IpProtocol: aws.String("tcp"),
		ToPort:     aws.Int64(2049),
	}
	if _, err := ec2svc.AuthorizeSecurityGroupIngress(asgiInput); err != nil {
		panic(err)
	}
}

// getFileSystems returns the current state of operator-managed file systems and
// access points as a `fileSystems` structure.
func getFileSystems(efssvc *efs.EFS) fileSystems {
	fsret, err := efssvc.DescribeFileSystems(nil)
	if err != nil {
		panic(err)
	}
	fsmap := make(fileSystems)
	for _, fsd := range fsret.FileSystems {
		creationToken := *fsd.CreationToken
		chunks := strings.Split(creationToken, ":")
		if len(chunks) != 2 || chunks[0] != fsTokenMarker {
			continue
		}
		fsKey := chunks[1]
		fsid := fsd.FileSystemId
		dapInput := &efs.DescribeAccessPointsInput{
			FileSystemId: fsid,
		}
		apret, err := efssvc.DescribeAccessPoints(dapInput)
		if err != nil {
			panic(err)
		}
		aps := make(accessPoints)
		for _, ap := range apret.AccessPoints {
			clientToken := *ap.ClientToken
			chunks := strings.Split(clientToken, ":")
			if len(chunks) != 2 || chunks[0] != apTokenMarker {
				continue
			}
			aps[chunks[1]] = *ap.AccessPointId
		}
		fsmap[fsKey] = fileSystem{
			fileSystemID:       *fsid,
			lastLifeCycleState: *fsd.LifeCycleState,
			accessPoints:       aps,
		}
	}
	return fsmap
}

// newFileSystem creates a new EFS file system
func newFileSystem(efssvc *efs.EFS, subnetIDs []*string, sgid string, ownedTag *efs.Tag, key string) string {
	log.Info("Creating file system...", "key", key)
	fsInput := &efs.CreateFileSystemInput{
		CreationToken: aws.String(fmt.Sprintf("%s:%s", fsTokenMarker, key)),
		// TODO(efried): Make this configurable
		Encrypted: aws.Bool(true),
		Tags:      []*efs.Tag{ownedTag},
	}
	fsd, err := efssvc.CreateFileSystem(fsInput)
	if err != nil {
		panic(err)
	}
	fsid := *fsd.FileSystemId
	log.Info("Created new file system", "fsid", fsid)

	return fsid
}

func waitForFSAvailable(efssvc *efs.EFS, fs fileSystem) {
	dfsInput := &efs.DescribeFileSystemsInput{
		FileSystemId: &fs.fileSystemID,
	}
	for fs.lastLifeCycleState != "available" {
		time.Sleep(time.Second)
		dfsOutput, err := efssvc.DescribeFileSystems(dfsInput)
		if err != nil {
			panic(err)
		}
		if len(dfsOutput.FileSystems) != 1 {
			panic(fmt.Sprintf("Expected exactly one file system for ID %s but found %d",
				fs.fileSystemID, len(dfsOutput.FileSystems)))
		}
		fs.lastLifeCycleState = *dfsOutput.FileSystems[0].LifeCycleState
	}
}

func ensureMountTargets(efssvc *efs.EFS, fsid string, subnetIDs []*string, sgid string) {
	log.Info("Ensuring mount targets...", "fsid", fsid)
	seen := make(map[string]bool)
	for _, subnetID := range subnetIDs {
		if exists := seen[*subnetID]; exists {
			continue
		}
		seen[*subnetID] = true
		ensureMountTarget(efssvc, fsid, *subnetID, sgid)
	}
}

func ensureMountTarget(efssvc *efs.EFS, fsid string, subnetID string, sgid string) {
	cmtInput := &efs.CreateMountTargetInput{
		FileSystemId:   aws.String(fsid),
		SecurityGroups: []*string{aws.String(sgid)},
		SubnetId:       aws.String(subnetID),
	}
	if mtDesc, err := efssvc.CreateMountTarget(cmtInput); err == nil {
		log.Info("Created mount target", "mount target ID", *mtDesc.MountTargetId)
	} else if _, ok := err.(*efs.MountTargetConflict); ok {
		log.Info("Mount target already exists for subnet", "subnet ID", subnetID)
	} else {
		panic(err)
	}
}

func newAccessPoint(efssvc *efs.EFS, fsid string, key string) string {
	log.Info("Creating access point...", "fsid", fsid, "key", key)
	apInput := &efs.CreateAccessPointInput{
		ClientToken:  aws.String(fmt.Sprintf("%s:%s", apTokenMarker, key)),
		FileSystemId: aws.String(fsid),
		RootDirectory: &efs.RootDirectory{
			CreationInfo: &efs.CreationInfo{
				// TODO(efried): Make these customizable
				OwnerGid:    aws.Int64(0),
				OwnerUid:    aws.Int64(0),
				Permissions: aws.String("775"),
			},
			// Use the key, which is unique within this file system, as the subdirectory
			Path: aws.String(fmt.Sprintf("/%s", key)),
		},
	}
	apOutput, err := efssvc.CreateAccessPoint(apInput)
	if err != nil {
		panic(err)
	}
	return *apOutput.AccessPointId
}

func deleteMountTargets(efssvc *efs.EFS, fsid string) {
	log.Info("Deleting mount targets", "fsid", fsid)
	descInput := &efs.DescribeMountTargetsInput{
		FileSystemId: aws.String(fsid),
	}
	// First send a delete request to all the mount targets
	descOutput, err := efssvc.DescribeMountTargets(descInput)
	if err != nil {
		panic(err)
	}
	if len(descOutput.MountTargets) == 0 {
		// They're all gone already
		return
	}
	for _, mt := range descOutput.MountTargets {
		log.Info("Deleting mount target.", "mount target ID", *mt.MountTargetId)
		delInput := &efs.DeleteMountTargetInput{
			MountTargetId: mt.MountTargetId,
		}
		if _, err := efssvc.DeleteMountTarget(delInput); err != nil {
			if _, ok := err.(*efs.MountTargetNotFound); !ok {
				panic(err)
			}
		}
	}

	// Now we need to poll until the mount targets are gone
	for {
		// The first time through, we've *just* sent all the delete requests, so putting this
		// sleep up front makes sense.
		time.Sleep(time.Second * 2)

		descOutput, err := efssvc.DescribeMountTargets(descInput)
		if err != nil {
			panic(err)
		}
		if len(descOutput.MountTargets) == 0 {
			// They're all gone
			break
		}
		mtList := make([]string, len(descOutput.MountTargets))
		for i, mt := range descOutput.MountTargets {
			mtList[i] = *mt.MountTargetId
		}
		log.Info("Waiting for mount targets.", "mount target IDs", mtList)
	}
}

func deleteFileSystem(efssvc *efs.EFS, fsid string) {
	deleteMountTargets(efssvc, fsid)
	log.Info("Removing file system...", "fsid", fsid)
	dfsInput := &efs.DeleteFileSystemInput{
		FileSystemId: aws.String(fsid),
	}
	if _, err := efssvc.DeleteFileSystem(dfsInput); err != nil {
		panic(fmt.Sprintf("Couldn't remove file system %s: %v", fsid, err))
	}
}

func deleteAccessPoint(efssvc *efs.EFS, fsid string, apid string) {
	log.Info("Removing access point...", "fsid", fsid, "apid", apid)
	dapInput := &efs.DeleteAccessPointInput{
		AccessPointId: aws.String(apid),
	}
	if _, err := efssvc.DeleteAccessPoint(dapInput); err != nil {
		panic(fmt.Sprintf("Couldn't remove access point %s: %v", apid, err))
	}
}

func deleteEverything() {
	sess := getSession()
	efssvc := getEFS(sess)
	currentState := getFileSystems(efssvc)
	for _, currentfs := range currentState {
		deleteFileSystem(efssvc, currentfs.fileSystemID)
	}
}

func discoverPrint() {
	sess := getSession()
	efssvc := getEFS(sess)
	currentState := getFileSystems(efssvc)
	for _, fs := range currentState {
		for _, ap := range fs.accessPoints {
			fmt.Printf("%s:%s\n", fs.fileSystemID, ap)
		}
	}
}

func ensureFileSystemState(desired fileSystems) {
	// Set up API services
	sess := getSession()
	ec2svc := getEC2(sess)
	efssvc := getEFS(sess)

	// Gather info about worker instances
	workers := getWorkers(ec2svc)
	sgid := getSecurityGroupID(workers)
	subnetIDs := getSubnetIDs(workers)
	ec2Tag := getOwnedTag(workers)
	ensureNFSIngressRule(ec2svc, sgid)

	// We'll use the "same" owned tag for file systems as for worker nodes
	efsTag := tagEC2ToEFS(ec2Tag)

	// Map the current state of operator-managed file systems and access points
	currentState := getFileSystems(efssvc)

	// Now reconcile the current state with the desired state
	// First remove any extraneous file systems.
	for fskey, currentfs := range currentState {
		if _, ok := desired[fskey]; ok {
			// This fs was desired; keep it

			// Reconcile access points.
			// First remove any extraneous access points.
			for apkey, currentap := range currentfs.accessPoints {
				if _, ok := desired[fskey].accessPoints[apkey]; !ok {
					deleteAccessPoint(efssvc, currentfs.fileSystemID, currentap)
				}
			}

			// Now create any access points that don't exist yet
			for apkey := range desired[fskey].accessPoints {
				if _, ok := currentfs.accessPoints[apkey]; ok {
					// Access point exists
					continue
				}
				apid := newAccessPoint(efssvc, currentfs.fileSystemID, apkey)
				currentfs.accessPoints[apkey] = apid
			}

		} else {
			deleteFileSystem(efssvc, currentfs.fileSystemID)
		}
	}

	// Now create any file systems that don't exist yet
	for fskey, desiredfs := range desired {
		var fsid string
		currentfs, ok := currentState[fskey]
		if ok {
			fsid = currentfs.fileSystemID
		} else {
			fsid = newFileSystem(efssvc, subnetIDs, sgid, efsTag, fskey)
			currentfs = fileSystem{
				fileSystemID: fsid,
				accessPoints: make(accessPoints),
			}
			currentState[fskey] = currentfs
		}
		waitForFSAvailable(efssvc, currentfs)
		ensureMountTargets(efssvc, fsid, subnetIDs, sgid)

		for apkey := range desiredfs.accessPoints {
			if _, ok := currentfs.accessPoints[apkey]; ok {
				// Access point already exists
				continue
			}
			apid := newAccessPoint(efssvc, fsid, apkey)
			currentfs.accessPoints[apkey] = apid
		}
	}
}
