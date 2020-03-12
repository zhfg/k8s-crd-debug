# 简介 
Debugger是一个Kubernetes CRD项目，通过CRD controller 实现了向CRD中指定的deployment中附加一个Container.


# 使用方法
** 新建CRD **

<p>
<code>
apiVersion: k8s.family-zhang.com/v1
kind: DebugerType
metadata: 
  name: superman-hello1
  namespace: test
spec:
  deploymentName: "nginx"
  shareVolumeName: "share-data"
</code>
</p>
