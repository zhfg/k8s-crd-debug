# 简介 
Debugger是一个Kubernetes CRD项目，通过CRD controller 实现了向CRD中指定的deployment中附加一个Container.

细节如下：
|Filed|作用|
|-|-|
|deploymentName|用于指明需要将sidecar附加到的deployment的名称|
|-|-|
|shareVolumeName|用于指明共享文件的挂截路径|

# 使用方法
