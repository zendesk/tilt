import React from 'react'
import {TiltVersionConsistencyPrompt} from "./TiltVersionConsistencyCheck"

export default {
  title: "New UI/Shared/TiltVersionConsistencyModal"
}

export const normal = () =>
  <TiltVersionConsistencyPrompt frontendVersion="1.2.3" backendVersion="4.5.6"/>

