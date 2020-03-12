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

