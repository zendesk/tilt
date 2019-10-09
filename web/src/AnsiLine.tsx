import React from "react"
import Ansi from "ansi-to-react"

type AnsiLineProps = {
  line: string
  id: string
}

let AnsiLine = React.memo(function(props: AnsiLineProps) {
  return (
    <Ansi linkify={false} useClasses={true} className={props.id}>
      {props.line}
    </Ansi>
  )
})

export default AnsiLine
