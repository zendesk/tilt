import React, { Component } from "react"
import { ReactComponent as LogoWordmarkSvg } from "./assets/svg/logo-wordmark-gray.svg"
import AnsiLine from "./AnsiLine"
import "./LogPane.scss"
import ReactDOM from "react-dom"

const WHEEL_DEBOUNCE_MS = 250

type LogPaneProps = {
  log: string
  message?: string
  isExpanded: boolean
  podID: string
  endpoints: string[]
  podStatus: string
}
type LogPaneState = {
  autoscroll: boolean
  lastWheelEventTimeMs: number
}

class LogPane extends Component<LogPaneProps, LogPaneState> {
  private lastElement: HTMLDivElement | null = null
  private rafID: number | null = null

  constructor(props: LogPaneProps) {
    super(props)

    this.state = {
      autoscroll: true,
      lastWheelEventTimeMs: 0,
    }

    this.refreshAutoScroll = this.refreshAutoScroll.bind(this)
    this.handleWheel = this.handleWheel.bind(this)
    this.handleSelectionChange = this.handleSelectionChange.bind(this)
  }

  componentDidMount() {
    if (this.lastElement !== null) {
      this.lastElement.scrollIntoView()
    }

    window.addEventListener("scroll", this.refreshAutoScroll, { passive: true })
    window.addEventListener("wheel", this.handleWheel, { passive: true })
    document.addEventListener("selectionchange", this.handleSelectionChange, { passive: true })
  }

  private handleSelectionChange() {
    // TODO restrict to only things in the scrolling log view
    // TODO store on state
    // TODO differentiate the thing currently selected for the local case (ie I'm a user and I'm using Tilt to create a highlighted snapshot
    // vs, I'm the snapshot server and I'm telling the JavaScript what is highlighted
    let selection = document.getSelection()
    if (selection) {
      let beginning = selection.focusNode
      let end = selection.anchorNode
      let node = ReactDOM.findDOMNode(this)

      if (beginning && end && node && node.contains(beginning) && node.contains(end) && !node.isEqualNode(beginning) && !node.isEqualNode(end)) {
        // @ts-ignore
        let firstID = beginning.parentNode.parentNode.className
        // @ts-ignore
        let lastID = end.parentNode.parentNode.className
        console.log("First ID: " + firstID)
        console.log("Last ID: " + lastID)
      }
    }
  }

  componentDidUpdate() {
    if (!this.state.autoscroll) {
      return
    }
    if (this.lastElement) {
      this.lastElement.scrollIntoView()
    }
  }

  componentWillUnmount() {
    window.removeEventListener("scroll", this.refreshAutoScroll)
    window.removeEventListener("wheel", this.handleWheel)
    document.onselectionchange = () => {}
    if (this.rafID) {
      clearTimeout(this.rafID)
    }
  }

  private handleWheel(event: WheelEvent) {
    if (event.deltaY < 0) {
      this.setState({ autoscroll: false, lastWheelEventTimeMs: Date.now() })
    }
  }

  private refreshAutoScroll() {
    if (this.rafID) {
      cancelAnimationFrame(this.rafID)
    }

    this.rafID = requestAnimationFrame(() => {
      let autoscroll = this.computeAutoScroll()
      this.setState(prevState => {
        let lastWheelEventTimeMs = prevState.lastWheelEventTimeMs
        if (lastWheelEventTimeMs) {
          if (Date.now() - lastWheelEventTimeMs < WHEEL_DEBOUNCE_MS) {
            return prevState
          }
          return { autoscroll: false, lastWheelEventTimeMs: 0 }
        }

        return { autoscroll, lastWheelEventTimeMs: 0 }
      })
    })
  }

  // Compute whether we should auto-scroll from the state of the DOM.
  // This forces a layout, so should be used sparingly.
  computeAutoScroll() {
    // Always auto-scroll when we're recovering from a loading screen.
    if (!this.props.log || !this.lastElement) {
      return true
    }

    // Never auto-scroll if we're horizontally scrolled.
    if (window.scrollX) {
      return false
    }

    let lastElInView =
      this.lastElement.getBoundingClientRect().bottom < window.innerHeight
    return lastElInView
  }

  render() {
    let classes = `LogPane ${this.props.isExpanded ? "LogPane--expanded" : ""}`

    let log = this.props.log
    if (!log || log.length === 0) {
      return (
        <section className={classes}>
          <section className="Pane-empty-message">
            <LogoWordmarkSvg />
            <h2>No Logs Found</h2>
          </section>
        </section>
      )
    }

    let podID = this.props.podID
    let podStatus = this.props.podStatus
    let podIDEl = podID && (
      <>
        <div className="resourceInfo">
          <div className="resourceInfo-label">Pod Status:</div>
          <div className="resourceInfo-value">{podStatus}</div>
        </div>
        <div className="resourceInfo">
          <div className="resourceInfo-label">Pod ID:</div>
          <div className="resourceInfo-value">{podID}</div>
        </div>
      </>
    )

    let endpoints = this.props.endpoints
    let endpointsEl = endpoints.length > 0 && (
      <div className="resourceInfo">
        <div className="resourceInfo-label">
          Endpoint{endpoints.length > 1 ? "s" : ""}:
        </div>
        {endpoints.map(ep => (
          <a className="resourceInfo-value" href={ep} target="_blank" key={ep}>
            {ep}
          </a>
        ))}
      </div>
    )

    let logLines: Array<React.ReactElement> = []
    let lines = log.split("\n")
    logLines = lines.map(
      (line: string, i: number): React.ReactElement => {
        let key = "logLine" + i
        return <AnsiLine key={"logLine" + i} line={line} className={key} />
      }
    )
    logLines.push(
      <p
        key="logEnd"
        className="logEnd"
        ref={el => {
          this.lastElement = el
        }}
      >
        &#9608;
      </p>
    )

    return (
      <section className={classes}>
        {(endpoints || podID) && (
          <section className="resourceBar">
            {podIDEl}
            {endpointsEl}
          </section>
        )}
        <section className="logText">{logLines}</section>
      </section>
    )
  }
}

export default LogPane
