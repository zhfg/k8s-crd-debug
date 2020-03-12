# 简介 
Debugger是一个Kubernetes CRD项目，通过CRD controller 实现了向CRD中指定的deployment中附加一个Container.


# 使用方法
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

```
+ **稍等30秒，再次查看其Pod中的container的数量**
