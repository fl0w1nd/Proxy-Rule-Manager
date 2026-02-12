import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "neu-badge inline-flex items-center shrink-0 [&>svg]:size-3 gap-1 [&>svg]:pointer-events-none transition-[color,box-shadow] overflow-hidden",
  {
    variants: {
      variant: {
        default: "",
        active: "neu-badge-active",
        destructive:
          "!bg-destructive/10 !text-destructive !border-destructive/20",
        secondary: "",
        outline: "",
        blue: "neu-badge-blue",
        rose: "neu-badge-rose",
        amber: "neu-badge-amber",
        violet: "neu-badge-violet",
        teal: "neu-badge-teal",
        emerald: "neu-badge-emerald",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

function Badge({
  className,
  variant,
  asChild = false,
  ...props
}: React.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot : "span"

  return (
    <Comp
      data-slot="badge"
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  )
}

export { Badge, badgeVariants }
