class UserDataController < ApplicationController
  def export
    response.headers["Cache-Control"] = "private, no-store"
    send_data JSON.pretty_generate(UserDataExport.new(Current.user).call),
      filename: "spotreba-#{Time.current.utc.strftime('%Y%m%d-%H%M%S')}.json",
      type: "application/json", disposition: "attachment"
  end

  def import
    file = params[:file]
    unless file.is_a?(ActionDispatch::Http::UploadedFile) && file.size.between?(1, UserDataImport::MAX_BYTES)
      redirect_to root_path, alert: "Import: vyberte JSON súbor do 10 MiB.", status: :see_other
      return
    end

    result = UserDataImport.new(Current.user, file.read(UserDataImport::MAX_BYTES + 1)).call
    message = "Import: #{result.imported.values.sum} nových, #{result.skipped.values.sum} preskočených záznamov."
    reasons = {
      duplicate: "Duplicity", ambiguous_vehicle: "Nejednoznačné vozidlá",
      rule_configuration: "Rozdielne nastavenia pripomienok", dependency: "Závislé záznamy"
    }
    result.reasons.each { |reason, count| message += " #{reasons.fetch(reason)}: #{count}." if count.positive? }
    if result.normalized_notifications.positive?
      message += " Čakajúce upozornenia označené ako preskočené: #{result.normalized_notifications}."
    end
    redirect_to root_path, notice: message, status: :see_other
  rescue UserDataImport::InvalidFile => error
    redirect_to root_path, alert: "Import zlyhal: #{error.message}", status: :see_other
  rescue ActiveRecord::ActiveRecordError
    redirect_to root_path, alert: "Import zlyhal. Žiadne dáta neboli zmenené. Skúste znova.", status: :see_other
  end
end
