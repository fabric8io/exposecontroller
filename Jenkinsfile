#!/usr/bin/groovy
@Library('github.com/rawlingsj/fabric8-pipeline-library@master')
def dummy
goNode{
  dockerNode{
    def v = goRelease{
      githubOrganisation = 'fabric8io'
      dockerOrganisation = 'fabric8'
      project = 'exposecontroller'
    }

    stage ('Update downstream dependencies') {
      updateDownstreamDependencies(v)
    }
  }
}

def updateDownstreamDependencies(v) {
  pushPomPropertyChangePR {
    propertyName = 'exposecontroller.version'
    projects = [
            'fabric8io/fabric8-devops'
    ]
    version = v
  }
}