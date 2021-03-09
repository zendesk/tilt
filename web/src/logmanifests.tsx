import React, {Dispatch, SetStateAction, useContext, useState} from "react"
import {AllManifests, ManifestSet} from "./LogStore"

type LogManifestAccessor = {
  get: () => ManifestSet
  set: Dispatch<SetStateAction<ManifestSet>>
}

const logManifestContext = React.createContext<LogManifestAccessor>( { get: () => AllManifests, set: null })

export function useLogManifests() {
  const [manifests, setManifests] = useState(AllManifests)
  return <logManifestContext.Provider value= {{ manifests, setManifests }} ></logManifestContext.Provider>
}
