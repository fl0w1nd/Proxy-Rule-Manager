import * as React from "react"
import { Slot } from "@radix-ui/react-slot"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap text-sm font-medium transition-all duration-150 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 shrink-0 [&_svg]:shrink-0 outline-none cursor-pointer",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground rounded-xl border border-primary shadow-xs hover:bg-primary/90 active:bg-primary/85 focus-visible:ring-[3px] focus-visible:ring-primary/20",
        success:
          "bg-success text-white rounded-xl border border-success shadow-xs hover:bg-success/90 active:bg-success/85 dark:text-success-foreground",
        destructive:
          "bg-transparent text-destructive rounded-xl border border-destructive/30 hover:bg-destructive/8 active:bg-destructive/12 focus-visible:ring-[3px] focus-visible:ring-destructive/20",
        outline:
          "bg-background/80 text-foreground rounded-xl border border-border shadow-xs hover:bg-accent hover:border-border-strong active:bg-surface-strong focus-visible:ring-[3px] focus-visible:ring-ring/15",
        secondary:
          "bg-secondary text-secondary-foreground rounded-xl border border-border shadow-xs hover:bg-accent active:bg-surface-strong",
        ghost:
          "rounded-lg hover:bg-accent hover:text-accent-foreground active:bg-surface-strong",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default: "h-9 px-4 py-2",
        sm: "h-8 gap-1.5 px-3",
        lg: "h-10 px-6",
        icon: "size-9",
        "icon-sm": "size-8",
        "icon-lg": "size-10",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }
