require "test_helper"

class UserDataControllerTest < ActionDispatch::IntegrationTest
  setup do
    @user = vehicles(:one).user
    sign_in_as @user
  end

  test "exports an authenticated private JSON attachment" do
    get export_user_data_path, params: { user_id: vehicles(:two).user_id }
    assert_response :success
    assert_equal "application/json", response.media_type
    assert_match(/attachment.*spotreba-.*\.json/, response.headers["Content-Disposition"])
    assert_includes response.headers["Cache-Control"], "no-store"
    assert_equal @user.email_address, response.parsed_body["account"]["email_address"]
    assert_equal @user.vehicles.pluck(:id).sort, response.parsed_body["vehicles"].map { |vehicle| vehicle["id"] }.sort
  end

  test "imports a multipart export and reports duplicates" do
    assert_no_difference("Vehicle.count") do
      post import_user_data_path, params: { file: upload(UserDataExport.new(@user).call.to_json) }
    end
    assert_response :see_other
    assert_redirected_to root_path
    assert_includes flash[:notice], "Duplicity"
    assert_includes flash[:notice], "0 nových"
    assert_includes flash[:notice], "preskočených záznamov."
  end

  test "rejects malformed and missing uploads without writing" do
    assert_no_difference("Vehicle.count") do
      post import_user_data_path, params: { file: upload("{") }
    end
    assert_redirected_to root_path
    assert_equal "Import zlyhal: Neplatný JSON alebo príliš hlboké vnorenie.", flash[:alert]
    post import_user_data_path
    assert_equal "Import: vyberte JSON súbor do 10 MiB.", flash[:alert]
  end

  test "places import export and logout in the user submenu" do
    get root_path
    assert_select ".user-menu button[aria-controls='user-menu-panel'][aria-expanded='false']", 1
    assert_select ".user-menu-panel a[href='#{export_user_data_path}'][data-turbo='false']", 1
    assert_select ".user-menu-panel form[action='#{import_user_data_path}'][enctype='multipart/form-data'] input[type='file']", 1
    assert_select ".user-menu-panel form[action='#{session_path}'] input[value='delete']", 1
  end

  test "requires authentication for both endpoints" do
    delete session_path
    get export_user_data_path
    assert_redirected_to new_session_path
    post import_user_data_path, params: { file: upload("{}") }
    assert_redirected_to new_session_path
    get new_session_path
    assert_select ".user-menu", 0
  end

  private
    def upload(json)
      Rack::Test::UploadedFile.new(StringIO.new(json), "application/json", original_filename: "backup.json")
    end
end
