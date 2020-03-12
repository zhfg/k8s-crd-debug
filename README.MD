# 简介 
Debugger是一个Kubernetes CRD项目，通过CRD controller 实现了向CRD中指定的deployment中附加一个Container.

此项目使用了[kubernetes/code-generate]https://github.com/kubernetes/code-generate 被用来生成部分结构代码.

代码编写过程中参考了
https://itnext.io/how-to-generate-client-codes-for-kubernetes-custom-resource-definitions-crd-b4b9907769ba


# 使用方法
**运行**
```
./k8s-crd-debug -kubeconfig .kube/config
```
**新建CRD**
```
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:  
  name: debugertypes.k8s.family-zhang.com
spec:  
  group: k8s.family-zhang.com
  version: v1
  scope: Namespaced  
  names:    
    kind: DebugerType
    shortNames: 
    - dt
    plural: debugertypes
    singular: debugertype
```

**新建一个DebugerType资源**

```
apiVersion: k8s.family-zhang.com/v1
kind: DebugerType
metadata: 
  name: superman-hello1
  namespace: test
spec:
  deploymentName: "nginx"
  shareVolumeName: "share-data"
```

# 验证
+ **在test namespaces下新建一个名称nginx的deployment**
```
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    run: nginx
  name: nginx
  namespace: test
spec:
  replicas: 1
  selector:
    matchLabels:
      run: nginx
  strategy:
    rollingUpdate:
      maxSurge: 25%
      maxUnavailable: 25%
    type: RollingUpdate
  template:
    metadata:
      labels:
        run: nginx
    spec:
      containers:
      - image: nginx
        name: nginx
      - image: theiaide/theia:0.16.1
        imagePullPolicy: IfNotPresent
        name: sidecar-theia
        volumeMounts:
        - mountPath: /data/
          name: share-data
      volumes:
      - emptyDir: {}
        name: data
      - emptyDir: {}
        name: share-data
```
+ **查看其pod中的container的数量**
```
[root@kube01 ~]# kubectl get pods -n test
NAME                     READY   STATUS    RESTARTS   AGE
nginx-58f5b7bd88-z777x   1/1     Running   0          5h8m
```
+ **稍等30秒，再次查看其Pod中的container的数量**
```
[root@kube01 ~]# kubectl get pods -n test
NAME                     READY   STATUS    RESTARTS   AGE
nginx-58f5b7bd88-z777x   2/2     Running   0          5h8m
```
