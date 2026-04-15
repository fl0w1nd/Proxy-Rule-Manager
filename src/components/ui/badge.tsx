import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex items-center shrink-0 rounded-full border px-2.5 py-0.5 text-[11px] font-medium [&>svg]:size-3 gap-1 [&>svg]:pointer-events-none transition-colors overflow-hidden",
  {
    variants: {
      variant: {
        default:
          "bg-surface-subtle border-border text-muted-foreground",
        active: "badge-active",
        destructive:
          "bg-destructive/8 border-destructive/20 text-destructive",
        secondary:
          "bg-surface-subtle border-border text-muted-foreground",
        outline:
          "bg-transparent border-border text-muted-foreground",
        blue: "badge-tone-blue",
        rose: "badge-tone-rose",
        amber: "badge-tone-amber",
        violet: "badge-tone-violet",
        teal: "badge-tone-teal",
        emerald: "badge-tone-emerald",
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
