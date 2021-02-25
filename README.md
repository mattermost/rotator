# Rotator 

Rotator is a tool meant to smooth and accelerate k8s cluster upgrades and node rotations. It offers automation on autoscaling group recognition and flexility on options such as, how fast to rotate nodes, drain retries, waiting time between rotations and drains as well as mater/worker node separation. 

## How to use

The rotator tool can be imported either as a go package and used in any app that needs to rotate/detach/drain k8s nodes or used as a cli tool, where the rotator server can accept rotation requests.

### Import as Go package

To import as a Go package both the [rotator]("github.com/mattermost/rotator/rotator") and the [model]("github.com/mattermost/rotator/model") should be imported. 

The rotator should be called with a cluster object like the one bellow:

```golang
clusterRotator := rotatorModel.Cluster{
		ClusterID:            <The id of the cluster to rotate nodes>, (string)
		MaxScaling:           <Maximum number of nodes to rotate in each rotation>, (int)
		RotateMasters:        <if master nodes should be rotated>, (bool)
		RotateWorkers:        <if worker nodes should be rotated>, (bool)
		MaxDrainRetries:      <max number of retries when a node drain fails>, (int)
		EvictGracePeriod:     <pod evict grace period>, (int)
		WaitBetweenRotations: <wait between each rotation of groups of nodes defined by MaxScaling in seconds>, (int)
		WaitBetweenDrains:    <wait between each node drain in a group of nodes>, (int)
		ClientSet:            <k8s clientset>, (*kubernetes.Clientset)
	}
```

Calling the `InitRotateCluster` function of the rotator package with the defined clusterRotator object is all is needed to rotate a cluster. Example can be seen bellow:

```golang
rotatorMetadata, err = rotator.InitRotateCluster(&clusterRotator, rotatorMetadata)
	if err != nil {
		cluster.ProvisionerMetadataKops.RotatorRequest.Status = rotatorMetadata
		return err
	}
```

where 

```golang
rotatorMetadata = &rotator.RotatorMetadata{}
```

The node rotator returns metadata that in case of rotation failure include information of ASGs pending rotation. This metadata can be passed back to the InitRotateCluster and the rotator will resume from where it left. 


### Use Rotator as CLI tool

The Rotator can be used as a docker image or as a local server. 

#### Building

Simply run the following:

```bash
go install ./cmd/rotator
alias cloud='$HOME/go/bin/rotator'
```

#### Running

Run the server with:

```bash
rotator server
```

In a different terminal/window, to rotate a cluster:
```bash
rotator cluster rotate --cluster <cluster_id> --rotate-workers --rotate-masters --wait-between-rotations 30 --wait-between-drains 60 --max-scaling 4 --evict-grace-period 30
```

You will get a response like this one:
```bash
[{
    "ClusterID": "<cluster_id>",
    "MaxScaling": 4,
    "RotateMasters": true,
    "RotateWorkers": true,
    "MaxDrainRetries": 10,
    "EvictGracePeriod": 30,
    "WaitBetweenRotations": 30,
    "WaitBetweenDrains": 30,
    "ClientSet": null
}
```

### Other Setup

For the rotator to run access to both the AWS account and the K8s cluster is required to be able to do actions such as, `DescribeInstances`, `DetachInstances`, `TerminateInstances`, `DescribeAutoScalingGroups`, as well as `drain`, `kill`, `evict` pods, etc.

The relevant AWS Access and Secret key pair should be exported and k8s access should be provided via a passed clientset or a locally exported k8s config. 
