import * as React from "react"
import { cn } from "cn"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        "form-control h-9 w-full px-3 py-1 text-base placeholder:text-muted-foreground read-only:bg-background md:text-sm",
        className
      )}
      {...props}
    />
  )
}

export { Input }
