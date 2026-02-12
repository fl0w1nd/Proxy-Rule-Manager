import type { LucideIcon } from "lucide-react"
import { Card } from "@/components/ui/card"
import { Button } from "@/components/ui/button"

interface EmptyStateProps {
    icon: LucideIcon
    title: string
    description: string
    action?: {
        label: string
        onClick: () => void
        icon?: LucideIcon
    }
    className?: string
}

export function EmptyState({
    icon: Icon,
    title,
    description,
    action,
    className,
}: EmptyStateProps) {
    return (
        <Card className={className}>
            <div className="text-center py-16 px-5">
                <div className="w-20 h-20 mx-auto mb-6 rounded-2xl bg-gradient-to-br from-muted/50 to-muted flex items-center justify-center">
                    <Icon className="w-10 h-10 text-muted-foreground/40" />
                </div>
                <p className="text-lg font-medium text-foreground">{title}</p>
                <p className="text-sm text-muted-foreground mt-2 max-w-sm mx-auto">
                    {description}
                </p>
                {action && (
                    <Button onClick={action.onClick} className="mt-6">
                        {action.icon && <action.icon className="w-4 h-4 mr-2" />}
                        {action.label}
                    </Button>
                )}
            </div>
        </Card>
    )
}
