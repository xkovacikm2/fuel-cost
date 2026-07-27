require "test_helper"

class RegistrationsControllerTest < ActionDispatch::IntegrationTest
  test "should get new" do
    get new_registration_url
    assert_response :success
  end

  test "should create account" do
    assert_difference("User.count") do
      post registration_url, params: { user: { email_address: "new-user@example.com", password: "password", password_confirmation: "password" } }
    end

    assert_redirected_to root_url
  end
end
