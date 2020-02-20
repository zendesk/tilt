// Pod Status
const podStatusError = "Error"
const podStatusCrashLoopBackOff = "CrashLoopBackOff"
const podStatusImagePullBackOff = "ImagePullBackOff"
const podStatusErrImgPull = "ErrImagePull"
const podStatusRunError = "RunContainerError"
const podStatusStartError = "StartError"

function podStatusIsCrash(status: string | undefined) {
  return status === podStatusError || status === podStatusCrashLoopBackOff
}

function podStatusIsError(status: string | undefined) {
  return (
    status === podStatusError ||
    status === podStatusCrashLoopBackOff ||
    status === podStatusImagePullBackOff ||
    status === podStatusErrImgPull ||
    status === podStatusRunError ||
    status === podStatusStartError
  )
}

// UpdateType
const updateTypeUnspecified = "unspecified"
const updateTypeImage = "image"
const updateTypeLiveUpdate = "live update"
const updateTypeDockerCompose = "docker compose"
const updateTypeK8s = "k8s"
const updateTypeLocal = "local"

function updateTypeToString(ut: string): string {
  if (ut === "1") {
    return updateTypeImage
  } else if (ut === "2") {
    return updateTypeLiveUpdate
  } else if (ut === "3") {
    return updateTypeDockerCompose
  } else if (ut === "4") {
    return updateTypeK8s
  } else if (ut === "5") {
    return updateTypeLocal
  }
    return updateTypeUnspecified
}

function anyUpdateIsLiveUpdate(updateTypes: string[]): boolean {
  for (let i = 0; i < updateTypes.length; i++) {
    if (updateTypeToString(updateTypes[i]) === updateTypeLiveUpdate) {
      return true
    }
  }
  return false
}

// TargetType
const targetTypeUnspecified = "unspecified"
const targetTypeImage = "image"
const targetTypeK8s = "k8s"
const targetTypeDockerCompose = "docker compose"
const targetTypeLocal = "local"

function targetTypeToString(ut: string): string {
  if (ut === "1") {
    return targetTypeImage
  } else if (ut === "2") {
    return targetTypeK8s
  } else if (ut === "3") {
    return targetTypeDockerCompose
  } else if (ut === "4") {
    return targetTypeLocal
  }
  return targetTypeUnspecified
}

// ~~~ takes a proto.TargetSpec?
function anyTargetHasLiveUpdate(targets: string[]): boolean {
  // for (let i = 0; i < updateTypes.length; i++) {
  //   if (updateTypeToString(updateTypes[i]) === updateTypeLiveUpdate) {
  //     return true
  //   }
  // }
  return false
}

export { podStatusIsCrash, podStatusIsError }
