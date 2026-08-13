class MaintenanceReminderEvaluator
  Result = Struct.new(:queued_notification, :skipped_notifications, keyword_init: true)

  def initialize(rule, today: Date.current)
    @rule = rule
    @today = today
  end

  def evaluate
    @rule.with_lock do
      baseline = latest_matching_cost
      return Result.new(queued_notification: nil, skipped_notifications: []) unless baseline

      remaining = remaining_thresholds(baseline)
      return process_due_notification(baseline, remaining) if due?(remaining)

      process_advance_notification(baseline, remaining)
    end
  end

  private
    def latest_matching_cost
      @rule.latest_matching_cost(today: @today)
    end

    def remaining_thresholds(baseline)
      cycle = @rule.current_cycle(today: @today)

      {
        days: cycle.days_remaining,
        kilometres: cycle.kilometres_remaining
      }
    end

    def due?(remaining)
      (remaining[:days] && remaining[:days] <= 0) ||
        (remaining[:kilometres] && remaining[:kilometres] <= 0)
    end

    def process_due_notification(baseline, remaining)
      existing_due = notifications_for(baseline).find_by(notification_kind: :due)
      return Result.new(queued_notification: nil, skipped_notifications: []) if existing_due

      skipped_notifications = notifications_for(baseline).where(notification_kind: :advance, status: :queued).to_a
      skipped_notifications.each { |notification| notification.update!(status: :skipped) }

      skipped_notifications += missing_leads(baseline).map do |lead|
        create_notification(
          baseline,
          lead: lead,
          notification_kind: :advance,
          trigger_condition: lead.days_before.present? ? :date : :mileage,
          status: :skipped,
          remaining: remaining
        )
      end

      Result.new(
        queued_notification: create_notification(
          baseline,
          notification_kind: :due,
          trigger_condition: due_condition(remaining),
          status: :queued,
          remaining: remaining
        ),
        skipped_notifications: skipped_notifications
      )
    end

    def process_advance_notification(baseline, remaining)
      candidates = eligible_leads(baseline, remaining)
      return Result.new(queued_notification: nil, skipped_notifications: []) if candidates.empty?

      selected = candidates.min_by { |candidate| [ candidate[:progress], candidate[:lead_value], candidate[:lead].id ] }
      skipped_notifications = (candidates - [ selected ]).map do |candidate|
        create_notification(
          baseline,
          lead: candidate[:lead],
          notification_kind: :advance,
          trigger_condition: candidate[:condition],
          status: :skipped,
          remaining: remaining
        )
      end

      Result.new(
        queued_notification: create_notification(
          baseline,
          lead: selected[:lead],
          notification_kind: :advance,
          trigger_condition: selected[:condition],
          status: :queued,
          remaining: remaining
        ),
        skipped_notifications: skipped_notifications
      )
    end

    def eligible_leads(baseline, remaining)
      missing_leads(baseline).filter_map do |lead|
        if lead.days_before.present? && remaining[:days] <= lead.days_before
          candidate(lead, :date, remaining[:days], @rule.interval_days)
        elsif lead.kilometres_before.present? && remaining[:kilometres] <= lead.kilometres_before
          candidate(lead, :mileage, remaining[:kilometres], @rule.interval_km)
        end
      end
    end

    def candidate(lead, condition, remaining, interval)
      {
        lead: lead,
        condition: condition,
        progress: remaining.to_d / interval,
        lead_value: condition == :date ? lead.days_before : lead.kilometres_before
      }
    end

    def missing_leads(baseline)
      sent_or_skipped_lead_ids = notifications_for(baseline).where(notification_kind: :advance).pluck(:maintenance_reminder_lead_id)
      @rule.maintenance_reminder_leads.ordered.where.not(id: sent_or_skipped_lead_ids)
    end

    def notifications_for(baseline)
      @rule.maintenance_notifications.where(additional_cost: baseline)
    end

    def due_condition(remaining)
      return :date if remaining[:days] && remaining[:days] <= 0

      :mileage
    end

    def create_notification(baseline, lead: nil, notification_kind:, trigger_condition:, status:, remaining:)
      @rule.maintenance_notifications.create!(
        additional_cost: baseline,
        maintenance_reminder_lead: lead,
        notification_kind: notification_kind,
        trigger_condition: trigger_condition,
        status: status,
        days_remaining: remaining[:days],
        kilometres_remaining: remaining[:kilometres]
      )
    end
end
