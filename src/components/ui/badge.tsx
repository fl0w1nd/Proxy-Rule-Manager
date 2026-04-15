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
        active:
          "bg-primary-soft border-primary/20 text-primary",
        destructive:
          "bg-destructive/8 border-destructive/20 text-destructive",
        secondary:
          "bg-surface-subtle border-border text-muted-foreground",
        outline:
          "bg-transparent border-border text-muted-foreground",
        blue:
          "bg-[oklch(0.96_0.02_250)] border-[oklch(0.90_0.03_250)] text-[oklch(0.50_0.11_250)] dark:bg-[oklch(0.26_0.03_250)] dark:border-[oklch(0.35_0.04_250)] dark:text-[oklch(0.78_0.08_250)]",
        rose:
          "bg-[oklch(0.96_0.02_18)] border-[oklch(0.90_0.03_18)] text-[oklch(0.55_0.12_18)] dark:bg-[oklch(0.26_0.03_18)] dark:border-[oklch(0.35_0.04_18)] dark:text-[oklch(0.78_0.08_18)]",
        amber:
          "bg-[oklch(0.96_0.025_75)] border-[oklch(0.90_0.035_75)] text-[oklch(0.52_0.12_75)] dark:bg-[oklch(0.26_0.03_75)] dark:border-[oklch(0.35_0.04_75)] dark:text-[oklch(0.78_0.08_75)]",
        violet:
          "bg-[oklch(0.96_0.02_300)] border-[oklch(0.90_0.03_300)] text-[oklch(0.52_0.12_300)] dark:bg-[oklch(0.26_0.03_300)] dark:border-[oklch(0.35_0.04_300)] dark:text-[oklch(0.78_0.08_300)]",
        teal:
          "bg-[oklch(0.96_0.02_210)] border-[oklch(0.90_0.03_210)] text-[oklch(0.50_0.10_210)] dark:bg-[oklch(0.26_0.03_210)] dark:border-[oklch(0.35_0.04_210)] dark:text-[oklch(0.78_0.08_210)]",
        emerald:
          "bg-[oklch(0.96_0.02_155)] border-[oklch(0.90_0.03_155)] text-[oklch(0.48_0.10_155)] dark:bg-[oklch(0.26_0.03_155)] dark:border-[oklch(0.35_0.04_155)] dark:text-[oklch(0.78_0.08_155)]",
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
