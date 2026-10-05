class UserDataImport
  class InvalidFile < StandardError; end

  MAX_BYTES = 10.megabytes
  MAX_RECORDS = 100_000
  MAX_DEPTH = 32
  MODELS = {
    vehicles: Vehicle, refuelings: Refueling, additional_costs: AdditionalCost,
    maintenance_reminder_rules: MaintenanceReminderRule,
    maintenance_reminder_leads: MaintenanceReminderLead, maintenance_notifications: MaintenanceNotification
  }.freeze
  CHILDREN = {
    vehicles: [ :refuelings, :additional_costs, :maintenance_reminder_rules, :maintenance_notifications ],
    maintenance_reminder_rules: [ :maintenance_reminder_leads ]
  }.freeze
  MATCH_FIELDS = {
    vehicles: [ :name, :fuel_type ], refuelings: [ :refueled_on, :distance_km, :amount, :cost ],
    additional_costs: [ :occurred_on, :kind, :cost ], maintenance_reminder_rules: [ :kind ],
    maintenance_reminder_leads: [ :days_before, :kilometres_before ]
  }.freeze
  Node = Struct.new(:collection, :attributes, :record, :children, keyword_init: true)
  Result = Struct.new(:imported, :skipped, :reasons, :normalized_notifications, keyword_init: true)

  def initialize(user, json)
    @user = user
    @json = json
    @sources = MODELS.to_h { |collection, _model| [ collection, {} ] }
    @mapped = {}
    @record_count = 0
    @result = Result.new(imported: Hash.new(0), skipped: Hash.new(0), reasons: Hash.new(0), normalized_notifications: 0)
  end

  def call
    vehicles = validate_file
    ApplicationRecord.transaction do
      @user.lock!
      vehicles.each { |vehicle| import_vehicle(vehicle) }
    end
    @result
  end

  private
    def validate_file
      invalid!("file", "súbor chýba alebo je príliš veľký") unless @json.is_a?(String) && @json.bytesize.between?(1, MAX_BYTES)
      payload = JSON.parse(@json, max_nesting: MAX_DEPTH)
      check_keys(payload, %w[format version exported_at account vehicles], "root")
      unless payload["format"] == UserDataExport::FORMAT && payload["version"].is_a?(Integer) && payload["version"] == UserDataExport::VERSION
        invalid!("version", "nepodporovaný formát alebo verzia")
      end
      convert(payload["exported_at"], :timestamp, "exported_at")
      check_keys(payload["account"], %w[email_address created_at updated_at], "account")
      payload["account"].each do |field, value|
        convert(value, field == "email_address" ? :string : :timestamp, "account.#{field}")
      end
      array(payload["vehicles"], "vehicles").each_with_index.map do |data, index|
        build_vehicle(data, "vehicles[#{index}]")
      end
    rescue JSON::ParserError
      raise InvalidFile, "Neplatný JSON alebo príliš hlboké vnorenie."
    end

    def build_vehicle(data, path)
      vehicle = build_node(:vehicles, data, path, user: @user)
      %i[refuelings additional_costs].each do |collection|
        vehicle.children.concat(build_children(collection, data, path, vehicle: vehicle.record))
      end
      rules = array(data["maintenance_reminder_rules"], "#{path}.maintenance_reminder_rules").each_with_index.map do |rule_data, index|
        rule_path = "#{path}.maintenance_reminder_rules[#{index}]"
        rule = build_node(:maintenance_reminder_rules, rule_data, rule_path, vehicle: vehicle.record)
        rule.children.concat(build_children(:maintenance_reminder_leads, rule_data, rule_path, maintenance_reminder_rule: rule.record))
        rule
      end
      vehicle.children.concat(rules)
      array(data["maintenance_notifications"], "#{path}.maintenance_notifications").each_with_index do |notification_data, index|
        notification_path = "#{path}.maintenance_notifications[#{index}]"
        invalid!(notification_path) unless notification_data.is_a?(Hash)
        rule = @sources[:maintenance_reminder_rules][notification_data["maintenance_reminder_rule_id"]]
        cost = @sources[:additional_costs][notification_data["additional_cost_id"]]
        lead_id = notification_data["maintenance_reminder_lead_id"]
        lead = @sources[:maintenance_reminder_leads][lead_id] unless lead_id.nil?
        unless rule && cost && rule.record.vehicle.equal?(vehicle.record) && cost.record.vehicle.equal?(vehicle.record) && rule.record.kind == cost.record.kind
          invalid!(notification_path, "neplatné prepojenie pravidla alebo nákladu")
        end
        if !lead_id.nil? && (!lead || !lead.record.maintenance_reminder_rule.equal?(rule.record))
          invalid!(notification_path, "neplatné prepojenie predstihu")
        end
        vehicle.children << build_node(:maintenance_notifications, notification_data, notification_path,
          maintenance_reminder_rule: rule.record, additional_cost: cost.record, maintenance_reminder_lead: lead&.record)
      end
      vehicle
    end

    def build_children(collection, data, path, **associations)
      array(data[collection.to_s], "#{path}.#{collection}").each_with_index.map do |child, index|
        build_node(collection, child, "#{path}.#{collection}[#{index}]", **associations)
      end
    end

    def build_node(collection, data, path, **associations)
      @record_count += 1
      invalid!(path, "príliš veľa záznamov") if @record_count > MAX_RECORDS
      fields = UserDataExport::FIELDS.fetch(collection)
      check_keys(data, fields.keys.map(&:to_s) + CHILDREN.fetch(collection, []).map(&:to_s), path)
      attributes = fields.to_h { |field, type| [ field, convert(data[field.to_s], type, "#{path}.#{field}") ] }
      invalid!(path, "duplicitné zdrojové ID") if @sources[collection].key?(attributes[:id])
      record = MODELS.fetch(collection).new(writable_attributes(attributes).merge(associations))
      invalid!(path, "neplatné hodnoty záznamu") unless record.valid?
      node = Node.new(collection: collection, attributes: attributes, record: record, children: [])
      @sources[collection][attributes[:id]] = node
      node
    end

    def check_keys(data, keys, path)
      invalid!(path, "neplatné alebo chýbajúce polia") unless data.is_a?(Hash) && data.keys.sort == keys.sort
    end

    def array(value, path)
      invalid!(path, "očakávaný zoznam") unless value.is_a?(Array)
      value
    end

    def convert(value, type, path)
      if type.to_s.start_with?("optional_")
        return if value.nil?
        type = type.to_s.delete_prefix("optional_").to_sym
      end
      case type
      when :id, :integer, :positive_integer
        limit = type == :id ? (1..9_223_372_036_854_775_807) : (-2_147_483_648..2_147_483_647)
        invalid!(path) unless value.is_a?(Integer) && limit.cover?(value)
        invalid!(path) if type == :positive_integer && value <= 0
        value
      when :decimal, :positive_decimal
        invalid!(path) unless value.is_a?(String) && value.match?(/\A-?\d{1,8}(?:\.\d{1,2})?\z/)
        number = BigDecimal(value)
        invalid!(path) if type == :positive_decimal && number <= 0
        number
      when :string
        invalid!(path) unless value.is_a?(String) && value.strip.present? && value.length <= 1_000
        value
      when :boolean
        invalid!(path) unless value == true || value == false
        value
      when :date
        invalid!(path) unless value.is_a?(String) && value.match?(/\A\d{4}-\d{2}-\d{2}\z/)
        date = Date.iso8601(value)
        invalid!(path) unless date.year.positive? && date.iso8601 == value
        date
      when :timestamp
        invalid!(path) unless value.is_a?(String) && value.match?(/\A\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?(?:Z|[+-]\d{2}:\d{2})\z/)
        Date.iso8601(value.first(10))
        time = Time.iso8601(value)
        invalid!(path) unless time.year.between?(1, 9999) && time.strftime("%Y-%m-%dT%H:%M:%S") == value.first(19)
        time
      else
        enum = case type
        when :fuel_type then Vehicle.fuel_types
        when :kind then AdditionalCost.kinds
        else MaintenanceNotification.defined_enums.fetch(type.to_s)
        end
        invalid!(path) unless value.is_a?(String) && enum.key?(value)
        value
      end
    rescue ArgumentError
      invalid!(path)
    end

    def invalid!(path, reason = "neplatná hodnota")
      raise InvalidFile, "#{path}: #{reason}."
    end

    def writable_attributes(attributes)
      attributes.except(:id, :maintenance_reminder_rule_id, :maintenance_reminder_lead_id, :additional_cost_id)
    end

    def import_vehicle(node)
      matches = @user.vehicles.where(node.attributes.slice(*MATCH_FIELDS[:vehicles])).limit(2).to_a
      return skip_tree(node, :ambiguous_vehicle) if matches.size > 1

      vehicle = persist(node, @user.vehicles, matches.first)
      node.children.each do |child|
        case child.collection
        when :maintenance_reminder_rules then import_rule(child, vehicle)
        when :maintenance_notifications then import_notification(child)
        else import_record(child, vehicle.public_send(child.collection))
        end
      end
    end

    def import_rule(node, vehicle)
      scope = vehicle.maintenance_reminder_rules
      existing = scope.find_by(node.attributes.slice(:kind))
      rule = persist(node, scope, existing)
      if %i[interval_days interval_km active].any? { |field| rule.public_send(field) != node.attributes[field] }
        @mapped[[ node.collection, node.attributes[:id] ]] = nil
        @result.reasons[:duplicate] -= 1
        @result.reasons[:rule_configuration] += 1
        node.children.each { |lead| skip_tree(lead, :dependency) }
        return
      end
      node.children.each { |lead| import_record(lead, rule.maintenance_reminder_leads) }
    end

    def import_record(node, scope)
      persist(node, scope, scope.find_by(node.attributes.slice(*MATCH_FIELDS.fetch(node.collection))))
    end

    def import_notification(node)
      attributes = node.attributes
      rule = @mapped[[ :maintenance_reminder_rules, attributes[:maintenance_reminder_rule_id] ]]
      cost = @mapped[[ :additional_costs, attributes[:additional_cost_id] ]]
      lead = @mapped[[ :maintenance_reminder_leads, attributes[:maintenance_reminder_lead_id] ]] if attributes[:maintenance_reminder_lead_id]
      return skip_tree(node, :dependency) unless rule && cost && (attributes[:maintenance_reminder_lead_id].nil? || lead)

      relationships = { additional_cost: cost, maintenance_reminder_lead: lead }
      match = relationships.merge(notification_kind: attributes[:notification_kind])
      scope = rule.maintenance_notifications
      existing = scope.find_by(match)
      overrides = relationships
      if !existing && attributes[:status] == "queued"
        overrides = overrides.merge(status: "skipped")
      end
      previous_count = @result.imported[:maintenance_notifications]
      persist(node, scope, existing, **overrides)
      if attributes[:status] == "queued" && @result.imported[:maintenance_notifications] > previous_count
        @result.normalized_notifications += 1
      end
    end

    def persist(node, scope, existing, **overrides)
      record = existing
      unless record
        begin
          ApplicationRecord.transaction(requires_new: true) do
            record = scope.create!(writable_attributes(node.attributes).merge(overrides))
          end
        rescue ActiveRecord::RecordNotUnique
          fields = MATCH_FIELDS[node.collection]
          match = fields ? node.attributes.slice(*fields) : overrides.slice(:additional_cost, :maintenance_reminder_lead).merge(notification_kind: node.attributes[:notification_kind])
          record = scope.find_by!(match)
          existing = record
        end
      end
      counts = existing ? @result.skipped : @result.imported
      counts[node.collection] += 1
      @result.reasons[:duplicate] += 1 if existing
      @mapped[[ node.collection, node.attributes[:id] ]] = record
      record
    end

    def skip_tree(node, reason)
      @result.skipped[node.collection] += 1
      @result.reasons[reason] += 1
      @mapped[[ node.collection, node.attributes[:id] ]] = nil
      node.children.each { |child| skip_tree(child, :dependency) }
    end
end
