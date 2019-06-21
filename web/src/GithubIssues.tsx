import React from "react"
import { ReactComponent as GithubMarkSvg } from "./assets/svg/github-mark.svg"
import { ReactComponent as ChevronSvg } from "./assets/svg/chevron.svg"

const GithubIssuesButton = () => (
  <a
    className="AnalyticsNudge-button"
    href="https://github.com/windmilleng/tilt/issues"
    target="_blank"
  >
    <GithubMarkSvg /> Github issues
  </a>
)

export default GithubIssuesButton
